package discovery

import "testing"

func TestParseServiceAnnotations_Valid(t *testing.T) {
	t.Setenv("DASHBRR_RADARR_API_KEY", "annotation-key")

	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		GetLabelKey(labelTypeKey):   "radarr",
		GetLabelKey(labelURLKey):    "http://radarr.radarr.svc.cluster.local:80",
		GetLabelKey(labelAPIKeyKey): "${DASHBRR_RADARR_API_KEY}",
		GetLabelKey(labelNameKey):   "Movies",
	}
	service, err := k.parseServiceAnnotations(annotations, "radarr", "radarr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service == nil {
		t.Fatalf("expected discovered service")
	}
	if service.InstanceID != "radarr-k8s-radarr" {
		t.Fatalf("instance id = %q, want %q", service.InstanceID, "radarr-k8s-radarr")
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

func TestParseServiceAnnotations_IgnoresNonDiscoveryAnnotations(t *testing.T) {
	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		"tailscale.com/expose": "true",
	}

	service, err := k.parseServiceAnnotations(annotations, "sonarr", "sonarr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when annotations are missing")
	}
}

func TestParseServiceAnnotations_Disabled(t *testing.T) {
	k := &KubernetesDiscovery{}
	annotations := map[string]string{
		GetLabelKey(labelTypeKey):    "prowlarr",
		GetLabelKey(labelURLKey):     "http://prowlarr.prowlarr.svc.cluster.local:80",
		GetLabelKey(labelAPIKeyKey):  "key",
		GetLabelKey(labelEnabledKey): "false",
	}

	service, err := k.parseServiceAnnotations(annotations, "prowlarr", "prowlarr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when disabled")
	}
}

func TestParseServiceAnnotations_NoMetadata(t *testing.T) {
	k := &KubernetesDiscovery{}

	service, err := k.parseServiceAnnotations(nil, "default", "svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when no discovery metadata exists")
	}
}
