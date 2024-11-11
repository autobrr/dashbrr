package discovery

const (
	// labelPrefix is the common prefix for all dashbrr service labels
	labelPrefix = "com.dashbrr.service"

	// Common label suffixes
	labelTypeKey    = "type"    // Service type (e.g., radarr, sonarr)
	labelURLKey     = "url"     // Service URL
	labelAPIKeyKey  = "apikey"  // Service API key
	labelNameKey    = "name"    // Optional display name override
	labelEnabledKey = "enabled" // Optional service enabled state
)

// GetLabelKey returns the full label key for a given suffix
func GetLabelKey(suffix string) string {
	return labelPrefix + "." + suffix
}
