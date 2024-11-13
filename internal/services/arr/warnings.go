package arr

// WarningCategory represents the category of a warning message
type WarningCategory string

const (
	SystemCategory       WarningCategory = "System"
	DownloadCategory     WarningCategory = "Download"
	IndexerCategory      WarningCategory = "Indexer"
	MediaCategory        WarningCategory = "Media"
	NotificationCategory WarningCategory = "Notification"
	ApplicationCategory  WarningCategory = "Application"
)

// WarningPattern represents a known warning pattern and its categorization
type WarningPattern struct {
	Pattern  string
	Category WarningCategory
}

// Known warning patterns for *arr applications
var knownWarnings = []WarningPattern{
	// System warnings
	{Pattern: "Branch is not a valid release branch", Category: SystemCategory},
	{Pattern: "Update to .NET", Category: SystemCategory},
	{Pattern: "installed mono version", Category: SystemCategory},
	{Pattern: "installed SQLite version", Category: SystemCategory},
	{Pattern: "Database Failed Integrity", Category: SystemCategory},
	{Pattern: "update is available", Category: SystemCategory},
	{Pattern: "folder is not writable", Category: SystemCategory},
	{Pattern: "System Time is off", Category: SystemCategory},
	{Pattern: "connect to signalR", Category: SystemCategory},
	{Pattern: "resolve the IP Address", Category: SystemCategory},
	{Pattern: "Proxy Failed Test", Category: SystemCategory},
	{Pattern: "Branch is for a previous version", Category: SystemCategory},
	{Pattern: "app translocation folder", Category: SystemCategory},
	{Pattern: "Invalid API Key", Category: SystemCategory},
	{Pattern: "Recycling Bin", Category: SystemCategory},
	{Pattern: "Unable to reach", Category: SystemCategory},

	// Download client warnings
	{Pattern: "download client is available", Category: DownloadCategory},
	{Pattern: "communicate with download client", Category: DownloadCategory},
	{Pattern: "Download clients are unavailable", Category: DownloadCategory},
	{Pattern: "Completed Download Handling", Category: DownloadCategory},
	{Pattern: "Docker bad remote path", Category: DownloadCategory},
	{Pattern: "Downloading into Root", Category: DownloadCategory},
	{Pattern: "Bad Download Client", Category: DownloadCategory},
	{Pattern: "Remote Path Mapping", Category: DownloadCategory},
	{Pattern: "download client requires sorting", Category: DownloadCategory},
	{Pattern: "Unable to reach Freebox", Category: DownloadCategory},

	// Indexer warnings
	{Pattern: "indexers available", Category: IndexerCategory},
	{Pattern: "indexers are enabled", Category: IndexerCategory},
	{Pattern: "Indexers are unavailable", Category: IndexerCategory},
	{Pattern: "Jackett All Endpoint", Category: IndexerCategory},
	{Pattern: "indexer unavailable due to failures", Category: IndexerCategory},
	{Pattern: "RSS sync enabled", Category: IndexerCategory},
	{Pattern: "automatic search enabled", Category: IndexerCategory},
	{Pattern: "interactive search enabled", Category: IndexerCategory},
	{Pattern: "No Definition", Category: IndexerCategory},
	{Pattern: "Indexers are Obsolete", Category: IndexerCategory},
	{Pattern: "Obsolete due to", Category: IndexerCategory},
	{Pattern: "VIP Expiring", Category: IndexerCategory},
	{Pattern: "VIP Expired", Category: IndexerCategory},
	{Pattern: "Long-term indexer", Category: IndexerCategory},
	{Pattern: "Unable to connect to indexer", Category: IndexerCategory},
	{Pattern: "check your DNS settings", Category: IndexerCategory},
	{Pattern: "Search failed", Category: IndexerCategory},

	// Application warnings (Prowlarr specific)
	{Pattern: "Applications are unavailable", Category: ApplicationCategory},
	{Pattern: "applications are unavailable", Category: ApplicationCategory},

	// Media warnings
	{Pattern: "Root Folder", Category: MediaCategory},
	{Pattern: "removed from TMDb", Category: MediaCategory},
	{Pattern: "removed from TheTVDB", Category: MediaCategory},
	{Pattern: "Mount is Read Only", Category: MediaCategory},
	{Pattern: "Import List", Category: MediaCategory},
	{Pattern: "Lists are unavailable", Category: MediaCategory},
	{Pattern: "Test was aborted", Category: MediaCategory},

	// Notification warnings
	{Pattern: "Notifications unavailable", Category: NotificationCategory},
	{Pattern: "Discord as Slack", Category: NotificationCategory},

	// Proxy warnings
	{Pattern: "resolve proxy", Category: SystemCategory},
	{Pattern: "proxy failed", Category: SystemCategory},
}
