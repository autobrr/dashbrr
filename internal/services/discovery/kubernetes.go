package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/autobrr/dashbrr/internal/models"
)

// KubernetesDiscovery handles service discovery from Kubernetes metadata.
type KubernetesDiscovery struct {
	client *kubernetes.Clientset
}

// NewKubernetesDiscovery creates a new Kubernetes discovery instance
func NewKubernetesDiscovery() (*KubernetesDiscovery, error) {
	// Prefer in-cluster auth when running inside Kubernetes.
	config, err := rest.InClusterConfig()
	if err != nil {
		inClusterErr := err

		// Fallback to kubeconfig for local CLI usage.
		var kubeconfig string
		if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
			kubeconfig = envKubeconfig
		} else if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		if kubeconfig == "" {
			return nil, fmt.Errorf("failed to load in-cluster config and no kubeconfig found: %w", inClusterErr)
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load in-cluster config (%v) and kubeconfig (%s): %w", inClusterErr, kubeconfig, err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &KubernetesDiscovery{
		client: clientset,
	}, nil
}

// DiscoverServices finds services configured via Kubernetes metadata
func (k *KubernetesDiscovery) DiscoverServices(ctx context.Context) ([]models.ServiceConfiguration, error) {
	// List all services, then filter in-memory. Annotation selectors are not supported.
	services, err := k.client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var configurations []models.ServiceConfiguration

	for _, service := range services.Items {
		config, err := k.parseServiceAnnotations(service.Annotations, service.Namespace, service.Name)
		if err != nil {
			log.Warn().
				Err(err).
				Str("namespace", service.Namespace).
				Str("service", service.Name).
				Msg("Failed to parse discovery metadata")
			continue
		}
		if config != nil {
			configurations = append(configurations, *config)
		}
	}

	return configurations, nil
}

// parseServiceAnnotations extracts service configuration from Kubernetes annotations.
func (k *KubernetesDiscovery) parseServiceAnnotations(annotations map[string]string, namespace, serviceName string) (*models.ServiceConfiguration, error) {
	if annotations[GetLabelKey(labelTypeKey)] == "" {
		return nil, nil
	}

	parsed, err := parseDiscoveryLabels(annotations)
	if err != nil {
		return nil, fmt.Errorf("invalid discovery metadata for %s/%s: %w", namespace, serviceName, err)
	}
	if !parsed.enabled {
		return nil, nil
	}

	instanceID := fmt.Sprintf("%s-k8s-%s-%s", parsed.serviceType, namespace, serviceName)

	return &models.ServiceConfiguration{
		InstanceID:  instanceID,
		DisplayName: parsed.displayName,
		URL:         parsed.url,
		APIKey:      parsed.apiKey,
	}, nil
}

// Close is a no-op for Kubernetes client
func (k *KubernetesDiscovery) Close() error {
	return nil
}
