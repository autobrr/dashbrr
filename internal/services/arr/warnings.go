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
	{Pattern: "Update to .NET version", Category: SystemCategory},
	{Pattern: "Currently installed mono version", Category: SystemCategory},
	{Pattern: "Currently installed SQLite version", Category: SystemCategory},
	{Pattern: "Database Failed Integrity Check", Category: SystemCategory},
	{Pattern: "New update is available", Category: SystemCategory},
	{Pattern: "Startup folder is not writable", Category: SystemCategory},
	{Pattern: "System Time is off", Category: SystemCategory},
	{Pattern: "Could not connect to signalR", Category: SystemCategory},

	// Additional System warnings
	{Pattern: "Branch is for a previous version", Category: SystemCategory},
	{Pattern: "Failed to resolve the IP Address", Category: SystemCategory},
	{Pattern: "Proxy Failed Test", Category: SystemCategory},

	// Download client warnings
	{Pattern: "No download client is available", Category: DownloadCategory},
	{Pattern: "Unable to communicate with download client", Category: DownloadCategory},
	{Pattern: "Download clients are unavailable", Category: DownloadCategory},
	{Pattern: "Completed Download Handling", Category: DownloadCategory},
	{Pattern: "Docker bad remote path mapping", Category: DownloadCategory},
	{Pattern: "Downloading into Root Folder", Category: DownloadCategory},
	{Pattern: "Bad Download Client Settings", Category: DownloadCategory},
	{Pattern: "Bad Remote Path Mapping", Category: DownloadCategory},

	// Indexer warnings
	{Pattern: "No indexers available", Category: IndexerCategory},
	{Pattern: "No indexers are enabled", Category: IndexerCategory},
	{Pattern: "Indexers are unavailable", Category: IndexerCategory},
	{Pattern: "Jackett All Endpoint Used", Category: IndexerCategory},

	// Additional Indexer warnings
	{Pattern: "Indexers Have No Definition", Category: IndexerCategory},
	{Pattern: "Indexers are Obsolete", Category: IndexerCategory},
	{Pattern: "Obsolete due to Code Changes", Category: IndexerCategory},
	{Pattern: "Obsolete due to Site Removals", Category: IndexerCategory},
	{Pattern: "Indexer VIP Expiring", Category: IndexerCategory},
	{Pattern: "Indexer VIP Expired", Category: IndexerCategory},

	// New Application warnings (Prowlarr specific)
	{Pattern: "Applications are unavailable", Category: ApplicationCategory},

	// Media warnings
	{Pattern: "Missing Root Folder", Category: MediaCategory},
	{Pattern: "was removed from TMDb", Category: MediaCategory},
	{Pattern: "was removed from TheTVDB", Category: MediaCategory},
	{Pattern: "Series Path Mount is Read Only", Category: MediaCategory},
	{Pattern: "Import List Missing Root Folder", Category: MediaCategory},
	{Pattern: "Lists are unavailable", Category: MediaCategory},

	// Notification warnings
	{Pattern: "Notifications unavailable", Category: NotificationCategory},
	{Pattern: "Discord as Slack Notification", Category: NotificationCategory},
}
