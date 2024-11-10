package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/autobrr/dashbrr/internal/models"
)

// KubernetesDiscovery handles service discovery from Kubernetes labels
type KubernetesDiscovery struct {
	client *kubernetes.Clientset
}

// NewKubernetesDiscovery creates a new Kubernetes discovery instance
func NewKubernetesDiscovery() (*KubernetesDiscovery, error) {
	// Try to load kubeconfig from standard locations
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Allow overriding kubeconfig location via environment variable
	if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
		kubeconfig = envKubeconfig
	}

	// Create the config from kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
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

// DiscoverServices finds services configured via Kubernetes labels
func (k *KubernetesDiscovery) DiscoverServices(ctx context.Context) ([]models.ServiceConfiguration, error) {
	// List all services in all namespaces with dashbrr labels
	services, err := k.client.CoreV1().Services("").List(ctx, metav1.ListOptions{
		LabelSelector: GetLabelKey(labelTypeKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var configurations []models.ServiceConfiguration

	for _, service := range services.Items {
		config, err := k.parseServiceLabels(service.Labels, service.Namespace)
		if err != nil {
			fmt.Printf("Warning: Failed to parse labels for service %s/%s: %v\n",
				service.Namespace, service.Name, err)
			continue
		}
		if config != nil {
			configurations = append(configurations, *config)
		}
	}

	return configurations, nil
}

// parseServiceLabels extracts service configuration from Kubernetes labels
func (k *KubernetesDiscovery) parseServiceLabels(labels map[string]string, namespace string) (*models.ServiceConfiguration, error) {
	serviceType := labels[GetLabelKey(labelTypeKey)]
	if serviceType == "" {
		return nil, fmt.Errorf("service type label not found")
	}

	url := labels[GetLabelKey(labelURLKey)]
	if url == "" {
		return nil, fmt.Errorf("service URL label not found")
	}

	// Handle environment variable substitution in API key
	apiKey := labels[GetLabelKey(labelAPIKeyKey)]
	if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(apiKey, "${"), "}")
		apiKey = os.Getenv(envVar)
		if apiKey == "" {
			return nil, fmt.Errorf("environment variable %s not set for API key", envVar)
		}
	}

	// Get optional display name or use service type
	displayName := labels[GetLabelKey(labelNameKey)]
	if displayName == "" {
		displayName = strings.Title(serviceType)
	}

	// Check if service is explicitly disabled
	if enabled := labels[GetLabelKey(labelEnabledKey)]; enabled == "false" {
		return nil, nil
	}

	// Generate instance ID based on service type and namespace
	instanceID := fmt.Sprintf("%s-k8s-%s", serviceType, namespace)

	return &models.ServiceConfiguration{
		InstanceID:  instanceID,
		DisplayName: displayName,
		URL:         url,
		APIKey:      apiKey,
	}, nil
}

// Close is a no-op for Kubernetes client
func (k *KubernetesDiscovery) Close() error {
	return nil
}
