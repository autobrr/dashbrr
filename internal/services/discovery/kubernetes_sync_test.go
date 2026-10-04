package discovery

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
)

func newSyncTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDBWithConfig(&database.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func k8sService(namespace, name string, annotations map[string]string) *corev1.Service {
	return &corev1.Service{Namespace: namespace, Name: name, Annotations: annotations}
}

func radarrAnnotations() map[string]string {
	return map[string]string{
		GetLabelKey(labelTypeKey):      "radarr",
		GetLabelKey(labelURLKey):       "http://radarr.media.svc.cluster.local:7878",
		GetLabelKey(labelAPIKeyKey):    "key",
		GetLabelKey(labelNameKey):      "Movies",
		GetLabelKey(labelAccessURLKey): "https://radarr.example.com",
	}
}

func servicesByID(t *testing.T, db *database.DB) map[string]models.ServiceConfiguration {
	t.Helper()
	services, err := db.GetAllServices(t.Context())
	if err != nil {
		t.Fatalf("get services: %v", err)
	}
	byID := make(map[string]models.ServiceConfiguration, len(services))
	for _, s := range services {
		s.ID = 0
		byID[s.InstanceID] = s
	}
	return byID
}

// seedNotOwned adds services that a sync must never change: one from the UI,
// one imported from a file, and one discovered before #145 changed the ID form.
func seedNotOwned(t *testing.T, db *database.DB) []models.ServiceConfiguration {
	t.Helper()
	seed := []models.ServiceConfiguration{
		{InstanceID: "radarr-1", DisplayName: "UI Radarr", URL: "http://radarr.local"},
		{InstanceID: "sonarr-config-1", DisplayName: "File Sonarr", URL: "http://sonarr.local", APIKey: "k"},
		{InstanceID: "radarr-k8s-media", DisplayName: "Old Radarr", URL: "http://radarr.media.svc.cluster.local:7878"},
	}
	for i := range seed {
		if err := db.CreateService(t.Context(), &seed[i]); err != nil {
			t.Fatalf("seed: %v", err)
		}
		seed[i].ID = 0
	}
	return seed
}

func assertNotOwnedUnchanged(t *testing.T, db *database.DB, seed []models.ServiceConfiguration) {
	t.Helper()
	got := servicesByID(t, db)
	for _, want := range seed {
		if got[want.InstanceID] != want {
			t.Fatalf("service %s changed: got %+v, want %+v", want.InstanceID, got[want.InstanceID], want)
		}
	}
}

func syncOrFail(t *testing.T, k *KubernetesDiscovery, db *database.DB) {
	t.Helper()
	if _, err := k.Sync(t.Context(), db); err != nil {
		t.Fatalf("sync: %v", err)
	}
}

func TestKubernetesSync_Lifecycle(t *testing.T) {
	ctx := t.Context()
	db := newSyncTestDB(t)
	seed := seedNotOwned(t, db)

	client := fake.NewClientset(
		k8sService("media", "radarr", radarrAnnotations()),
		k8sService("media", "sonarr", map[string]string{
			GetLabelKey(labelTypeKey): "sonarr",
			GetLabelKey(labelURLKey):  "http://sonarr.media.svc.cluster.local:8989",
		}),
	)
	k := &KubernetesDiscovery{client: client, dynamic: fakeDynamic(), namespaces: []string{"media"}}

	syncOrFail(t, k, db)
	got := servicesByID(t, db)
	wantRadarr := models.ServiceConfiguration{
		InstanceID:  "radarr-k8s-media.radarr",
		DisplayName: "Movies",
		URL:         "http://radarr.media.svc.cluster.local:7878",
		APIKey:      "key",
		AccessURL:   "https://radarr.example.com",
	}
	if got[wantRadarr.InstanceID] != wantRadarr {
		t.Fatalf("radarr = %+v, want %+v", got[wantRadarr.InstanceID], wantRadarr)
	}
	if got["sonarr-k8s-media.sonarr"].DisplayName != "Sonarr" {
		t.Fatalf("sonarr = %+v", got["sonarr-k8s-media.sonarr"])
	}
	assertNotOwnedUnchanged(t, db, seed)

	// A second sync with no change in the cluster changes nothing.
	plan, err := k.Sync(ctx, db)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !plan.Empty() {
		t.Fatalf("expected empty plan, got %+v", plan)
	}

	// Changed annotations update the record. A removed annotation clears its field.
	radarr := k8sService("media", "radarr", radarrAnnotations())
	radarr.Annotations[GetLabelKey(labelNameKey)] = "Films"
	delete(radarr.Annotations, GetLabelKey(labelAccessURLKey))
	if _, err := client.CoreV1().Services("media").Update(ctx, radarr, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update service: %v", err)
	}
	syncOrFail(t, k, db)
	got = servicesByID(t, db)
	if r := got["radarr-k8s-media.radarr"]; r.DisplayName != "Films" || r.AccessURL != "" {
		t.Fatalf("radarr after update = %+v", r)
	}

	// A removed type annotation deletes the record.
	if _, err := client.CoreV1().Services("media").Update(ctx, k8sService("media", "sonarr", nil), metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update service: %v", err)
	}
	syncOrFail(t, k, db)
	if _, ok := servicesByID(t, db)["sonarr-k8s-media.sonarr"]; ok {
		t.Fatalf("sonarr not deleted after type annotation removal")
	}

	// A deleted Service deletes the record.
	if err := client.CoreV1().Services("media").Delete(ctx, "radarr", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete service: %v", err)
	}
	syncOrFail(t, k, db)
	if _, ok := servicesByID(t, db)["radarr-k8s-media.radarr"]; ok {
		t.Fatalf("radarr not deleted after Service deletion")
	}
	assertNotOwnedUnchanged(t, db, seed)
}

func TestKubernetesSync_NamespaceScope(t *testing.T) {
	db := newSyncTestDB(t)
	client := fake.NewClientset(
		k8sService("media", "radarr", radarrAnnotations()),
		k8sService("other", "radarr", radarrAnnotations()),
	)

	k := &KubernetesDiscovery{client: client, dynamic: fakeDynamic(), namespaces: []string{""}}
	syncOrFail(t, k, db)
	got := servicesByID(t, db)
	if _, ok := got["radarr-k8s-other.radarr"]; !ok || len(got) != 2 {
		t.Fatalf("all namespaces: got %v", slices.Collect(maps.Keys(got)))
	}

	// A namespace that is no longer scanned loses its records.
	k.namespaces = []string{"media"}
	syncOrFail(t, k, db)
	got = servicesByID(t, db)
	if _, ok := got["radarr-k8s-other.radarr"]; ok || len(got) != 1 {
		t.Fatalf("media only: got %v", slices.Collect(maps.Keys(got)))
	}
}

func TestKubernetesSync_ListErrorChangesNothing(t *testing.T) {
	db := newSyncTestDB(t)
	client := fake.NewClientset(k8sService("media", "radarr", radarrAnnotations()))
	k := &KubernetesDiscovery{client: client, dynamic: fakeDynamic(), namespaces: []string{"media", "other"}}
	syncOrFail(t, k, db)

	client.PrependReactor("list", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "other" {
			return true, nil, errors.New("forbidden")
		}
		return false, nil, nil
	})
	if err := client.CoreV1().Services("media").Delete(t.Context(), "radarr", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete service: %v", err)
	}

	if _, err := k.Sync(t.Context(), db); err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := servicesByID(t, db)["radarr-k8s-media.radarr"]; !ok {
		t.Fatalf("failed list deleted a service")
	}
}

func TestResolveNamespaces(t *testing.T) {
	podFile := filepath.Join(t.TempDir(), "namespace")
	if err := os.WriteFile(podFile, []byte("media\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")

	tests := []struct {
		name       string
		configured []string
		file       string
		want       []string
		wantErr    bool
	}{
		{name: "empty in cluster", file: podFile, want: []string{"media"}},
		{name: "empty outside cluster", file: missing, wantErr: true},
		{name: "all namespaces", configured: []string{"*"}, file: missing, want: []string{""}},
		{name: "list", configured: []string{"media", "anime"}, file: missing, want: []string{"media", "anime"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveNamespaces(tt.configured, tt.file)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKubernetesSync_KeepsPlexToken(t *testing.T) {
	ctx := t.Context()
	db := newSyncTestDB(t)
	client := fake.NewClientset(k8sService("media", "plex", map[string]string{
		GetLabelKey(labelTypeKey): "plex",
		GetLabelKey(labelURLKey):  "http://plex.media.svc.cluster.local:32400",
	}))
	k := &KubernetesDiscovery{client: client, dynamic: fakeDynamic(), namespaces: []string{"media"}}
	syncOrFail(t, k, db)

	// The user saves a token with the Plex sign-in in the UI.
	plex := servicesByID(t, db)["plex-k8s-media.plex"]
	plex.APIKey = "oauth-token"
	if err := db.UpdateService(ctx, &plex); err != nil {
		t.Fatalf("update service: %v", err)
	}

	plan, err := k.Sync(ctx, db)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !plan.Empty() {
		t.Fatalf("expected empty plan, got %+v", plan)
	}

	// The sync ignores an apikey annotation on a Plex Service.
	annotated := k8sService("media", "plex", map[string]string{
		GetLabelKey(labelTypeKey):   "plex",
		GetLabelKey(labelURLKey):    "http://plex.media.svc.cluster.local:32400",
		GetLabelKey(labelAPIKeyKey): "annotated-token",
	})
	if _, err := client.CoreV1().Services("media").Update(ctx, annotated, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update service: %v", err)
	}
	syncOrFail(t, k, db)
	if got := servicesByID(t, db)["plex-k8s-media.plex"].APIKey; got != "oauth-token" {
		t.Fatalf("api key = %q, want %q", got, "oauth-token")
	}

	// An apikey annotation that does not resolve does not delete the Plex service.
	annotated.Annotations[GetLabelKey(labelAPIKeyKey)] = "${DASHBRR_TEST_UNSET_PLEX_TOKEN}"
	if _, err := client.CoreV1().Services("media").Update(ctx, annotated, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update service: %v", err)
	}
	syncOrFail(t, k, db)
	if got := servicesByID(t, db)["plex-k8s-media.plex"].APIKey; got != "oauth-token" {
		t.Fatalf("api key = %q, want %q", got, "oauth-token")
	}
}
