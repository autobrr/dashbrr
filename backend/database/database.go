package database

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/autobrr/dashbrr/backend/models"
	"github.com/autobrr/dashbrr/backend/types"
)

var db *sql.DB

// DB represents the database connection
type DB struct {
	*sql.DB
}

// InitDB initializes the database connection and performs migrations
func InitDB(dbPath string) (*DB, error) {
	// Ensure the database directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	log.Printf("Initializing database at: %s", dbPath)

	// Open database connection
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := database.Ping(); err != nil {
		return nil, err
	}

	// Create the services table if it doesn't exist
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS service_configurations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instance_id TEXT UNIQUE NOT NULL,
			display_name TEXT NOT NULL,
			url TEXT,
			api_key TEXT
		)
	`)
	if err != nil {
		return nil, err
	}

	// Create the users table if it doesn't exist
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}

	db = database
	return &DB{db}, nil
}

// HasUsers checks if any users exist in the database
func (db *DB) HasUsers() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// User Management Functions

// CreateUser creates a new user in the database
func (db *DB) CreateUser(user *types.User) error {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO users (username, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		user.Username,
		user.Email,
		user.PasswordHash,
		now,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = id
	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}

// GetUserByUsername retrieves a user by their username
func (db *DB) GetUserByUsername(username string) (*types.User, error) {
	var user types.User
	err := db.QueryRow(`
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE username = ?`,
		username,
	).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by their email
func (db *DB) GetUserByEmail(email string) (*types.User, error) {
	var user types.User
	err := db.QueryRow(`
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = ?`,
		email,
	).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (db *DB) GetUserByID(id int64) (*types.User, error) {
	var user types.User
	err := db.QueryRow(`
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = ?`,
		id,
	).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Service Management Functions

// GetServiceByInstanceID retrieves a service configuration by its instance ID
func (db *DB) GetServiceByInstanceID(instanceID string) (*models.ServiceConfiguration, error) {
	var service models.ServiceConfiguration
	err := db.QueryRow(`
		SELECT id, instance_id, display_name, url, api_key 
		FROM service_configurations 
		WHERE instance_id = ?`, instanceID).Scan(
		&service.ID,
		&service.InstanceID,
		&service.DisplayName,
		&service.URL,
		&service.APIKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// GetServiceByInstancePrefix retrieves a service configuration by its instance ID prefix
func (db *DB) GetServiceByInstancePrefix(prefix string) (*models.ServiceConfiguration, error) {
	var service models.ServiceConfiguration
	err := db.QueryRow(`
		SELECT id, instance_id, display_name, url, api_key 
		FROM service_configurations 
		WHERE instance_id LIKE ? || '%'
		LIMIT 1`, prefix).Scan(
		&service.ID,
		&service.InstanceID,
		&service.DisplayName,
		&service.URL,
		&service.APIKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// GetAllServices retrieves all service configurations
func (db *DB) GetAllServices() ([]models.ServiceConfiguration, error) {
	rows, err := db.Query(`
		SELECT id, instance_id, display_name, url, api_key 
		FROM service_configurations
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []models.ServiceConfiguration
	for rows.Next() {
		var service models.ServiceConfiguration
		err := rows.Scan(
			&service.ID,
			&service.InstanceID,
			&service.DisplayName,
			&service.URL,
			&service.APIKey,
		)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, nil
}

// CreateService creates a new service configuration
func (db *DB) CreateService(service *models.ServiceConfiguration) error {
	result, err := db.Exec(`
		INSERT INTO service_configurations (instance_id, display_name, url, api_key)
		VALUES (?, ?, ?, ?)`,
		service.InstanceID,
		service.DisplayName,
		service.URL,
		service.APIKey,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	service.ID = id
	return nil
}

// UpdateService updates an existing service configuration
func (db *DB) UpdateService(service *models.ServiceConfiguration) error {
	_, err := db.Exec(`
		UPDATE service_configurations 
		SET display_name = ?, url = ?, api_key = ?
		WHERE instance_id = ?`,
		service.DisplayName,
		service.URL,
		service.APIKey,
		service.InstanceID,
	)
	return err
}

// DeleteService deletes a service configuration by its instance ID
func (db *DB) DeleteService(instanceID string) error {
	_, err := db.Exec(`
		DELETE FROM service_configurations 
		WHERE instance_id = ?`,
		instanceID,
	)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
