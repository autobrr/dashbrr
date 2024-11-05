package models

// ServiceConfiguration is the database model
type ServiceConfiguration struct {
	ID          int64  `json:"-"` // Hide ID from JSON response
	InstanceID  string `json:"instanceId" gorm:"uniqueIndex"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	APIKey      string `json:"apiKey,omitempty"`
}
