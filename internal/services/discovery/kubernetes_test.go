package discovery

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
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
	service, err := k.parseService(k8sService("radarr", "radarr", annotations))
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
			service, err := k.parseService(k8sService(tt.namespace, tt.serviceName, annotations))
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

	service, err := k.parseService(k8sService("sonarr", "sonarr", annotations))
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

	service, err := k.parseService(k8sService("prowlarr", "prowlarr", annotations))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service when disabled")
	}
}

func TestParseService_NoMetadata(t *testing.T) {
	k := &KubernetesDiscovery{}

	service, err := k.parseService(k8sService("default", "svc", nil))
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

			service, err := k.parseService(svc)
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

	if _, err := k.parseService(svc); err == nil {
		t.Fatal("expected error for a service with no url annotation and no ports")
	}
}
