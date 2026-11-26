package discovery

import (
	"os"
	"testing"
)

func TestResolveEnvVar(t *testing.T) {
	t.Setenv("DASHBRR_TEST_KEY", "abc123")

	got, err := resolveEnvVar("${DASHBRR_TEST_KEY}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("got %q, want %q", got, "abc123")
	}
}

func TestResolveEnvVar_Missing(t *testing.T) {
	_ = os.Unsetenv("DASHBRR_TEST_MISSING")

	_, err := resolveEnvVar("${DASHBRR_TEST_MISSING}")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseContainerLabels_RejectsInvalidURL(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		wantErr string
	}{
		{
			name:    "url without scheme",
			labels:  map[string]string{GetLabelKey(labelURLKey): "admin:secret@nzbget:6789"}, //nolint:gosec // test fixture
			wantErr: "com.dashbrr.service.url: invalid service URL: must start with http:// or https://",
		},
		{
			name:    "access url without scheme",
			labels:  map[string]string{GetLabelKey(labelURLKey): "http://nzbget:6789", GetLabelKey(labelAccessURLKey): "nzbget.example.test"},
			wantErr: "com.dashbrr.service.access_url: invalid service URL: must start with http:// or https://",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.labels[GetLabelKey(labelTypeKey)] = "nzbget"
			service, err := (&DockerDiscovery{}).parseContainerLabels(tt.labels)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if service != nil {
				t.Fatalf("service = %+v, want none", service)
			}
		})
	}
}

func TestParseContainerLabels_TrimsURLs(t *testing.T) {
	service, err := (&DockerDiscovery{}).parseContainerLabels(map[string]string{
		GetLabelKey(labelTypeKey):      "nzbget",
		GetLabelKey(labelURLKey):       "http://nzbget:6789\n",
		GetLabelKey(labelAccessURLKey): " https://nzbget.example.test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service.URL != "http://nzbget:6789" {
		t.Fatalf("url = %q, want %q", service.URL, "http://nzbget:6789")
	}
}
