package discovery

import (
	"context"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/autobrr/dashbrr/internal/models"
)

// podNamespaceFile holds the namespace of the pod when dashbrr runs in a cluster.
const podNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// httpRouteGVR is the Gateway API HTTPRoute resource. Discovery reads it with
// the dynamic client, so dashbrr does not need the Gateway API Go module.
var httpRouteGVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

// KubernetesDiscovery handles service discovery from Kubernetes metadata.
type KubernetesDiscovery struct {
	client  kubernetes.Interface
	dynamic dynamic.Interface
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
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}

	return &KubernetesDiscovery{
		client:     clientset,
		dynamic:    dynamicClient,
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
		routeURLs, err := k.routeAccessURLs(ctx, namespace)
		if err != nil {
			return nil, err
		}

		for _, service := range services.Items {
			config, err := k.parseService(&service, routeURLs[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}])
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

// parseService extracts service configuration from the annotations of a Kubernetes Service.
// When the url annotation is empty, it infers the URL from the ports. When the
// access_url annotation is empty, it uses routeAccessURL.
func (k *KubernetesDiscovery) parseService(service *corev1.Service, routeAccessURL string) (*models.ServiceConfiguration, error) {
	if service.Annotations[GetLabelKey(labelTypeKey)] == "" {
		return nil, nil
	}
	annotations := maps.Clone(service.Annotations)
	// Plex gets its token from the Plex sign-in in the UI, so the sync ignores
	// an apikey annotation on Plex, even one that does not resolve.
	if annotations[GetLabelKey(labelTypeKey)] == "plex" {
		delete(annotations, GetLabelKey(labelAPIKeyKey))
	}
	if annotations[GetLabelKey(labelURLKey)] == "" {
		url := inferServiceURL(service)
		if url == "" {
			return nil, fmt.Errorf("service %s/%s has no url annotation and no ports to infer the URL from", service.Namespace, service.Name)
		}
		annotations[GetLabelKey(labelURLKey)] = url
	}
	if annotations[GetLabelKey(labelAccessURLKey)] == "" {
		annotations[GetLabelKey(labelAccessURLKey)] = routeAccessURL
	}

	parsed, err := parseDiscoveryLabels(annotations)
	if err != nil {
		return nil, fmt.Errorf("invalid discovery metadata for %s/%s: %w", service.Namespace, service.Name, err)
	}
	if !parsed.enabled {
		return nil, nil
	}

	return &models.ServiceConfiguration{
		InstanceID:  kubernetesInstanceID(parsed.serviceType, service.Namespace, service.Name),
		DisplayName: parsed.displayName,
		URL:         parsed.url,
		APIKey:      parsed.apiKey,
		AccessURL:   parsed.accessURL,
	}, nil
}

// inferServiceURL builds the in-cluster URL of a Service from its ports. It
// uses the port named https, else the port named http, else the first port.
// It returns "" when the Service has no ports.
func inferServiceURL(service *corev1.Service) string {
	ports := service.Spec.Ports
	if len(ports) == 0 {
		return ""
	}
	scheme, port := "http", ports[0].Port
	if i := slices.IndexFunc(ports, func(p corev1.ServicePort) bool { return p.Name == "https" }); i >= 0 {
		scheme, port = "https", ports[i].Port
	} else if i := slices.IndexFunc(ports, func(p corev1.ServicePort) bool { return p.Name == "http" }); i >= 0 {
		port = ports[i].Port
	}

	// The DNS search list of the pod resolves <service>.<namespace>.svc, so no cluster domain is necessary.
	host := service.Name + "." + service.Namespace + ".svc"
	if (scheme == "http" && port != 80) || (scheme == "https" && port != 443) {
		host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	}
	return scheme + "://" + host
}

// httpRoute holds the fields of a Gateway API HTTPRoute that discovery reads.
type httpRoute struct {
	Spec struct {
		Hostnames []string `json:"hostnames"`
		Rules     []struct {
			Matches []struct {
				Path struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"path"`
			} `json:"matches"`
			BackendRefs []struct {
				Group     string `json:"group"`
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Weight    *int32 `json:"weight"`
			} `json:"backendRefs"`
		} `json:"rules"`
	} `json:"spec"`
	Status struct {
		Parents []routeParentStatus `json:"parents"`
	} `json:"status"`
}

// routeParentStatus is the status that one parent Gateway writes on an HTTPRoute.
type routeParentStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

// rejected reports whether the route has a parent status and no parent
// accepted it. A route with no parent status counts as accepted, because a
// Gateway controller can be slow to write the status, or not write it.
func (r *httpRoute) rejected() bool {
	return len(r.Status.Parents) > 0 && !slices.ContainsFunc(r.Status.Parents, func(p routeParentStatus) bool {
		return meta.IsStatusConditionTrue(p.Conditions, "Accepted")
	})
}

// routeAccessURLs maps each Service to https://<hostname><path> of an HTTPRoute that
// has a backendRef to that Service in the namespace of the route. It uses the
// first hostname that is not a wildcard, and skips rejected routes and backendRefs with weight 0.
// The path comes from the first match of the rule when that match is a PathPrefix or an Exact path. When more than one route
// matches, it uses the route whose name is first in alphabetical order. When the HTTPRoute CRD is not installed or RBAC
// refuses the read, it logs at debug level and returns no URLs.
func (k *KubernetesDiscovery) routeAccessURLs(ctx context.Context, namespace string) (map[types.NamespacedName]string, error) {
	list, err := k.dynamic.Resource(httpRouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
		log.Debug().Err(err).Str("namespace", namespace).Msg("Cannot read HTTPRoutes, so discovery does not get access URLs from them")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list HTTPRoutes in namespace %q: %w", namespace, err)
	}

	slices.SortFunc(list.Items, func(a, b unstructured.Unstructured) int { return strings.Compare(a.GetName(), b.GetName()) })

	urls := make(map[types.NamespacedName]string)
	for _, item := range list.Items {
		var route httpRoute
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &route); err != nil {
			log.Warn().Err(err).Str("namespace", item.GetNamespace()).Str("route", item.GetName()).Msg("Failed to parse HTTPRoute")
			continue
		}
		// A browser cannot open a wildcard hostname.
		i := slices.IndexFunc(route.Spec.Hostnames, func(h string) bool { return !strings.HasPrefix(h, "*") })
		if i < 0 || route.rejected() {
			continue
		}
		for _, rule := range route.Spec.Rules {
			// A rule with no matches, or a path with no type, matches the prefix "/". A browser cannot open a regular expression, so it gets no path.
			var path string
			if len(rule.Matches) > 0 && slices.Contains([]string{"", "PathPrefix", "Exact"}, rule.Matches[0].Path.Type) {
				path = strings.TrimSuffix(rule.Matches[0].Path.Value, "/")
			}
			for _, ref := range rule.BackendRefs {
				// An empty group and kind mean a core Service. An empty namespace means the namespace of the route.
				// A backendRef with weight 0 gets no traffic.
				if ref.Group != "" || (ref.Kind != "" && ref.Kind != "Service") || (ref.Namespace != "" && ref.Namespace != item.GetNamespace()) ||
					(ref.Weight != nil && *ref.Weight == 0) {
					continue
				}
				key := types.NamespacedName{Namespace: item.GetNamespace(), Name: ref.Name}
				if _, ok := urls[key]; !ok {
					urls[key] = "https://" + route.Spec.Hostnames[i] + path
				}
			}
		}
	}
	return urls, nil
}

// kubernetesInstanceID returns the instance ID of a discovered service.
// Kubernetes namespaces and Service names cannot contain dots, so this boundary is unambiguous.
func kubernetesInstanceID(serviceType, namespace, serviceName string) string {
	return fmt.Sprintf("%s-k8s-%s.%s", serviceType, namespace, serviceName)
}
