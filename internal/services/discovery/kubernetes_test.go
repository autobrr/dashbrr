package discovery

import (
	"errors"
	"maps"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestParseService_Valid(t *testing.T) {
	t.Setenv("DASHBRR_RADARR_API_KEY", "annotation-key")

	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		GetLabelKey(labelTypeKey):   "radarr",
		GetLabelKey(labelURLKey):    "http://radarr.radarr.svc.cluster.local:80",
		GetLabelKey(labelAPIKeyKey): "${DASHBRR_RADARR_API_KEY}",
		GetLabelKey(labelNameKey):   "Movies",
	}
	service, err := k.parseService(k8sService("radarr", "radarr", annotations), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service == nil {
		t.Fatalf("expected discovered service")
	}
	if service.InstanceID != "radarr-k8s-radarr.radarr" {
		t.Fatalf("instance id = %q, want %q", service.InstanceID, "radarr-k8s-radarr.radarr")
	}
	if service.URL != "http://radarr.radarr.svc.cluster.local:80" {
		t.Fatalf("url = %q", service.URL)
	}
	if service.APIKey != "annotation-key" {
		t.Fatalf("api key = %q", service.APIKey)
	}
	if service.DisplayName != "Movies" {
		t.Fatalf("display name = %q", service.DisplayName)
	}
}

func TestParseService_InstanceID(t *testing.T) {
	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		GetLabelKey(labelTypeKey):   "sonarr",
		GetLabelKey(labelURLKey):    "http://sonarr.media.svc.cluster.local:80",
		GetLabelKey(labelAPIKeyKey): "key",
	}

	tests := []struct {
		name        string
		namespace   string
		serviceName string
		want        string
	}{
		{name: "same namespace first service", namespace: "media", serviceName: "sonarr", want: "sonarr-k8s-media.sonarr"},
		{name: "same namespace second service", namespace: "media", serviceName: "sonarr-anime", want: "sonarr-k8s-media.sonarr-anime"},
		{name: "hyphen boundary collision", namespace: "media-sonarr", serviceName: "anime", want: "sonarr-k8s-media-sonarr.anime"},
		{name: "same service again", namespace: "media", serviceName: "sonarr", want: "sonarr-k8s-media.sonarr"},
		{name: "different namespace", namespace: "anime", serviceName: "sonarr", want: "sonarr-k8s-anime.sonarr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := k.parseService(k8sService(tt.namespace, tt.serviceName, annotations), "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if service.InstanceID != tt.want {
				t.Fatalf("instance id = %q, want %q", service.InstanceID, tt.want)
			}
		})
	}
}

func TestParseService_IgnoresNonDiscoveryAnnotations(t *testing.T) {
	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		"tailscale.com/expose": "true",
	}

	service, err := k.parseService(k8sService("sonarr", "sonarr", annotations), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when annotations are missing")
	}
}

func TestParseService_Disabled(t *testing.T) {
	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		GetLabelKey(labelTypeKey):    "prowlarr",
		GetLabelKey(labelURLKey):     "http://prowlarr.prowlarr.svc.cluster.local:80",
		GetLabelKey(labelAPIKeyKey):  "key",
		GetLabelKey(labelEnabledKey): "false",
	}

	service, err := k.parseService(k8sService("prowlarr", "prowlarr", annotations), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when disabled")
	}
}

func TestParseService_DisabledSkipsValidation(t *testing.T) {
	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		GetLabelKey(labelTypeKey):    "prowlarr",
		GetLabelKey(labelAPIKeyKey):  "${DASHBRR_TEST_UNSET_API_KEY}",
		GetLabelKey(labelEnabledKey): "false",
	}

	service, err := k.parseService(k8sService("prowlarr", "prowlarr", annotations), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when disabled")
	}
}

func TestParseService_NoMetadata(t *testing.T) {
	k := &KubernetesDiscovery{}

	service, err := k.parseService(k8sService("default", "svc", nil), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when no discovery metadata exists")
	}
}

func TestParseService_InferredURL(t *testing.T) {
	k := &KubernetesDiscovery{}
	tests := []struct {
		name  string
		url   string
		ports []corev1.ServicePort
		want  string
	}{
		{
			name:  "http port on default port",
			ports: []corev1.ServicePort{{Name: "metrics", Port: 9090}, {Name: "http", Port: 80}},
			want:  "http://radarr.media.svc",
		},
		{
			name:  "https port wins over http",
			ports: []corev1.ServicePort{{Name: "http", Port: 80}, {Name: "https", Port: 8443}},
			want:  "https://radarr.media.svc:8443",
		},
		{
			name:  "https on default port",
			ports: []corev1.ServicePort{{Name: "https", Port: 443}},
			want:  "https://radarr.media.svc",
		},
		{
			name:  "first unnamed port",
			ports: []corev1.ServicePort{{Port: 7878}},
			want:  "http://radarr.media.svc:7878",
		},
		{
			name:  "url annotation overrides",
			url:   "http://radarr.example.test:7878",
			ports: []corev1.ServicePort{{Name: "http", Port: 80}},
			want:  "http://radarr.example.test:7878",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{GetLabelKey(labelTypeKey): "radarr"}
			if tt.url != "" {
				annotations[GetLabelKey(labelURLKey)] = tt.url
			}
			svc := k8sService("media", "radarr", annotations)
			svc.Spec.Ports = tt.ports

			service, err := k.parseService(svc, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if service.URL != tt.want {
				t.Fatalf("url = %q, want %q", service.URL, tt.want)
			}
		})
	}
}

func TestParseService_NoURLAndNoPorts(t *testing.T) {
	k := &KubernetesDiscovery{}
	svc := k8sService("media", "radarr", map[string]string{GetLabelKey(labelTypeKey): "radarr"})

	if _, err := k.parseService(svc, ""); err == nil {
		t.Fatal("expected error for a service with no url annotation and no ports")
	}
}

func newHTTPRoute(namespace, name string, hostnames []any, backends ...any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata":   map[string]any{"namespace": namespace, "name": name},
		"spec": map[string]any{
			"parentRefs": []any{map[string]any{"name": "gateway"}},
			"hostnames":  hostnames,
			"rules":      []any{map[string]any{"backendRefs": backends}},
		},
	}}
}

// withParents sets one parent status on the route for each value of the Accepted condition.
func withParents(route *unstructured.Unstructured, accepted ...string) *unstructured.Unstructured {
	parents := make([]any, len(accepted))
	for i, status := range accepted {
		parents[i] = map[string]any{"conditions": []any{map[string]any{"type": "Accepted", "status": status}}}
	}
	route.Object["status"] = map[string]any{"parents": parents}
	return route
}

// withPath adds one path match to the only rule of the route.
func withPath(route *unstructured.Unstructured, pathType, value string) *unstructured.Unstructured {
	rule := route.Object["spec"].(map[string]any)["rules"].([]any)[0].(map[string]any)
	matches, _ := rule["matches"].([]any)
	rule["matches"] = append(matches, map[string]any{"path": map[string]any{"type": pathType, "value": value}})
	return route
}

func withoutParentRefs(route *unstructured.Unstructured) *unstructured.Unstructured {
	delete(route.Object["spec"].(map[string]any), "parentRefs")
	return route
}

func fakeDynamic(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{httpRouteGVR: "HTTPRouteList"}, objects...)
}

func TestList_AccessURLFromHTTPRoute(t *testing.T) {
	radarr := map[string]string{GetLabelKey(labelTypeKey): "radarr", GetLabelKey(labelURLKey): "http://radarr.media.svc:7878"}
	withAnnotation := maps.Clone(radarr)
	withAnnotation[GetLabelKey(labelAccessURLKey)] = "https://movies.example.com"

	tests := []struct {
		name        string
		annotations map[string]string
		routes      []runtime.Object
		want        string
	}{
		{
			name:        "route hostname",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr"})},
			want:        "https://radarr.example.com",
		},
		{
			name:        "annotation overrides route",
			annotations: withAnnotation,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr"})},
			want:        "https://movies.example.com",
		},
		{
			name:        "first route by name",
			annotations: radarr,
			routes: []runtime.Object{
				newHTTPRoute("media", "b", []any{"b.example.com"}, map[string]any{"name": "radarr"}),
				newHTTPRoute("media", "a", []any{"a.example.com"}, map[string]any{"name": "radarr"}),
			},
			want: "https://a.example.com",
		},
		{
			name:        "route for another service",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "sonarr", []any{"sonarr.example.com"}, map[string]any{"name": "sonarr"})},
		},
		{
			name:        "route in another namespace",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("other", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr"})},
		},
		{
			name:        "backend in another namespace",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr", "namespace": "other"})},
		},
		{
			name:        "backend that is not a Service",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr", "kind": "Backend", "group": "example.com"})},
		},
		{
			name:        "wildcard hostname skipped",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", []any{"*.example.com", "radarr.example.com"}, map[string]any{"name": "radarr"})},
			want:        "https://radarr.example.com",
		},
		{
			name:        "only wildcard hostnames",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", []any{"*.example.com"}, map[string]any{"name": "radarr"})},
		},
		{
			name:        "rejected route skipped",
			annotations: radarr,
			routes: []runtime.Object{
				withParents(newHTTPRoute("media", "a", []any{"a.example.com"}, map[string]any{"name": "radarr"}), "False"),
				withParents(newHTTPRoute("media", "b", []any{"b.example.com"}, map[string]any{"name": "radarr"}), "False", "True"),
			},
			want: "https://b.example.com",
		},
		{
			name:        "path prefix match",
			annotations: radarr,
			routes:      []runtime.Object{withPath(newHTTPRoute("media", "radarr", []any{"media.example.com"}, map[string]any{"name": "radarr"}), "PathPrefix", "/radarr/")},
			want:        "https://media.example.com/radarr",
		},
		{
			name:        "root path match",
			annotations: radarr,
			routes:      []runtime.Object{withPath(newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr"}), "PathPrefix", "/")},
			want:        "https://radarr.example.com",
		},
		{
			name:        "regular expression path match",
			annotations: radarr,
			routes:      []runtime.Object{withPath(newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr"}), "RegularExpression", "/r.*")},
		},
		{
			name:        "first match that is not a regular expression",
			annotations: radarr,
			routes: []runtime.Object{withPath(withPath(newHTTPRoute("media", "radarr", []any{"media.example.com"}, map[string]any{"name": "radarr"}),
				"RegularExpression", "/r.*"), "PathPrefix", "/radarr")},
			want: "https://media.example.com/radarr",
		},
		{
			name:        "exact path keeps trailing slash",
			annotations: radarr,
			routes:      []runtime.Object{withPath(newHTTPRoute("media", "radarr", []any{"media.example.com"}, map[string]any{"name": "radarr"}), "Exact", "/radarr/")},
			want:        "https://media.example.com/radarr/",
		},
		{
			name:        "zero weight backend skipped",
			annotations: radarr,
			routes: []runtime.Object{
				newHTTPRoute("media", "a", []any{"a.example.com"}, map[string]any{"name": "radarr", "weight": int64(0)}),
				newHTTPRoute("media", "b", []any{"b.example.com"}, map[string]any{"name": "radarr", "weight": int64(1)}),
			},
			want: "https://b.example.com",
		},
		{
			name:        "route without parentRefs skipped",
			annotations: radarr,
			routes:      []runtime.Object{withoutParentRefs(newHTTPRoute("media", "radarr", []any{"radarr.example.com"}, map[string]any{"name": "radarr"}))},
		},
		{
			name:        "route without hostnames",
			annotations: radarr,
			routes:      []runtime.Object{newHTTPRoute("media", "radarr", nil, map[string]any{"name": "radarr"})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &KubernetesDiscovery{
				client:     fake.NewClientset(k8sService("media", "radarr", tt.annotations)),
				dynamic:    fakeDynamic(tt.routes...),
				namespaces: []string{metav1.NamespaceAll},
			}
			found, err := k.list(t.Context())
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(found) != 1 {
				t.Fatalf("found %d services, want 1", len(found))
			}
			if found[0].AccessURL != tt.want {
				t.Fatalf("access url = %q, want %q", found[0].AccessURL, tt.want)
			}
		})
	}
}

func TestList_HTTPRouteReadFails(t *testing.T) {
	gr := httpRouteGVR.GroupResource()
	tests := []struct {
		name string
		err  error
	}{
		{name: "CRD not installed", err: apierrors.NewNotFound(gr, "")},
		{name: "RBAC refuses", err: apierrors.NewForbidden(gr, "", errors.New("no access"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dyn := fakeDynamic()
			dyn.PrependReactor("list", "httproutes", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.err
			})
			k := &KubernetesDiscovery{
				client:     fake.NewClientset(k8sService("media", "radarr", map[string]string{GetLabelKey(labelTypeKey): "radarr", GetLabelKey(labelURLKey): "http://radarr.media.svc:7878"})),
				dynamic:    dyn,
				namespaces: []string{"media"},
			}
			found, err := k.list(t.Context())
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(found) != 1 || found[0].AccessURL != "" {
				t.Fatalf("found = %+v", found)
			}
		})
	}
}
