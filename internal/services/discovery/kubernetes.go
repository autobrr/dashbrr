package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/autobrr/dashbrr/internal/models"
)

// podNamespaceFile holds the namespace of the pod when dashbrr runs in a cluster.
const podNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// KubernetesDiscovery handles service discovery from Kubernetes metadata.
type KubernetesDiscovery struct {
	client kubernetes.Interface
	// namespaces to scan. An empty string means all namespaces.
	namespaces []string
}

// NewKubernetesDiscovery creates a Kubernetes discovery instance that scans the
// configured namespaces. "*" scans all namespaces. With no namespaces, it scans
// the namespace of the pod.
func NewKubernetesDiscovery(namespaces []string) (*KubernetesDiscovery, error) {
	scan, err := resolveNamespaces(namespaces, podNamespaceFile)
	if err != nil {
		return nil, err
	}

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
		client:     clientset,
		namespaces: scan,
	}, nil
}

// resolveNamespaces turns the configured namespaces into the namespaces to
// list. An empty list means the namespace of the pod, read from podFile.
func resolveNamespaces(configured []string, podFile string) ([]string, error) {
	if slices.Contains(configured, "*") {
		return []string{metav1.NamespaceAll}, nil
	}
	if len(configured) > 0 {
		return configured, nil
	}

	data, err := os.ReadFile(podFile)
	if err != nil {
		return nil, fmt.Errorf("kubernetes discovery namespaces are not set and the pod namespace is not available (%w): set DASHBRR__K8S_DISCOVERY_NAMESPACES, or use * for all namespaces", err)
	}
	return []string{strings.TrimSpace(string(data))}, nil
}

// list returns the services from the annotations in the scanned namespaces.
func (k *KubernetesDiscovery) list(ctx context.Context) ([]models.ServiceConfiguration, error) {
	var configurations []models.ServiceConfiguration

	for _, namespace := range k.namespaces {
		// List all services, then filter in-memory. Annotation selectors are not supported.
		services, err := k.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list services in namespace %q: %w", namespace, err)
		}

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

	return &models.ServiceConfiguration{
		InstanceID:  kubernetesInstanceID(parsed.serviceType, namespace, serviceName),
		DisplayName: parsed.displayName,
		URL:         parsed.url,
		APIKey:      parsed.apiKey,
		AccessURL:   parsed.accessURL,
	}, nil
}

// kubernetesInstanceID returns the instance ID of a discovered service.
// Kubernetes namespaces and Service names cannot contain dots, so this boundary is unambiguous.
func kubernetesInstanceID(serviceType, namespace, serviceName string) string {
	return fmt.Sprintf("%s-k8s-%s.%s", serviceType, namespace, serviceName)
}
