// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

// DB represents the database connection
type DB struct {
	*sql.DB
	driver string
}

// Config holds database configuration
type Config struct {
	Driver   string
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	Path     string // For SQLite
}

// NewConfig creates a new database configuration from environment variables
func NewConfig() *Config {
	dbType := os.Getenv("DASHBRR__DB_TYPE")
	if dbType == "" {
		dbType = "sqlite" // Default to SQLite
	}

	config := &Config{
		Driver: dbType,
	}

	if dbType == "postgres" {
		config.Host = getEnv("DASHBRR__DB_HOST", "localhost")
		config.Port = getEnv("DASHBRR__DB_PORT", "5432")
		config.User = getEnv("DASHBRR__DB_USER", "dashbrr")
		config.Password = getEnv("DASHBRR__DB_PASSWORD", "dashbrr")
		config.DBName = getEnv("DASHBRR__DB_NAME", "dashbrr")
	} else {
		config.Path = getEnv("DASHBRR__DB_PATH", "./data/dashbrr.db")
	}

	return config
}

// InitDB initializes the database connection and performs migrations
func InitDB(dbPath string) (*DB, error) {
	config := NewConfig()
	if config.Driver == "sqlite" {
		config.Path = dbPath
	}
	return InitDBWithConfig(config)
}

// InitDBWithConfig initializes the database with the provided configuration
func InitDBWithConfig(config *Config) (*DB, error) {
	var (
		database *sql.DB
		err      error
	)

	maxRetries := 5
	baseDelay := time.Second

	if config.Driver == "postgres" {
		// PostgreSQL connection
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.User, config.Password, config.DBName)
		log.Debug().
			Str("host", config.Host).
			Str("port", config.Port).
			Str("database", config.DBName).
			Msg("Initializing PostgreSQL database")

		// Retry loop with exponential backoff
		for attempt := 1; attempt <= maxRetries; attempt++ {
			database, err = sql.Open("postgres", dsn)
			if err == nil {
				// Test the connection
				err = database.Ping()
				if err == nil {
					break // Successfully connected
				}
			}

			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
			}

			delay := time.Duration(attempt) * baseDelay
			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying database connection")
			time.Sleep(delay)
		}
	} else {
		// SQLite connection
		dbDir := filepath.Dir(config.Path)
		// Create directory with restricted permissions
		if err := os.MkdirAll(dbDir, 0750); err != nil {
			return nil, err
		}

		// Create or open database
		database, err = sql.Open("sqlite", config.Path)
		if err != nil {
			return nil, fmt.Errorf("error opening database: %w", err)
		}

		// Force SQLite to create the database file by pinging it
		if err := database.Ping(); err != nil {
			return nil, fmt.Errorf("error creating database file: %w", err)
		}

		// Now that the file exists, set restrictive permissions
		if err := os.Chmod(config.Path, 0640); err != nil {
			return nil, fmt.Errorf("error setting database file permissions: %w", err)
		}
		log.Debug().
			Str("path", config.Path).
			Msg("Initializing SQLite database")
	}

	// Configure connection pool
	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(25)
	database.SetConnMaxLifetime(5 * time.Minute)

	log.Info().
		Str("driver", config.Driver).
		Msg("Successfully connected to database")

	db := &DB{database, config.Driver}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("error initializing schema: %w", err)
	}

	return db, nil
}

// initSchema creates the necessary database tables
func (db *DB) initSchema() error {
	var autoIncrement string
	if db.driver == "postgres" {
		autoIncrement = "SERIAL"
	} else {
		autoIncrement = "INTEGER"
	}

	// Create the services table
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS service_configurations (
			id %s PRIMARY KEY,
			instance_id TEXT UNIQUE NOT NULL,
			display_name TEXT NOT NULL,
			url TEXT,
			api_key TEXT
		)`, autoIncrement))
	if err != nil {
		return err
	}

	// Create the users table
	_, err = db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS users (
			id %s PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`, autoIncrement))
	if err != nil {
		return err
	}

	//log.Debug().Msg("Database schema initialized")
	return nil
}

// getEnv retrieves an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
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
	var result sql.Result
	var err error

	if db.driver == "postgres" {
		err = db.QueryRow(`
			INSERT INTO users (username, email, password_hash, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id`,
			user.Username,
			user.Email,
			user.PasswordHash,
			now,
			now,
		).Scan(&user.ID)
	} else {
		result, err = db.Exec(`
			INSERT INTO users (username, email, password_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			user.Username,
			user.Email,
			user.PasswordHash,
			now,
			now,
		)
		if err == nil {
			user.ID, err = result.LastInsertId()
		}
	}

	if err != nil {
		return err
	}

	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}

// GetUserByUsername retrieves a user by their username
func (db *DB) GetUserByUsername(username string) (*types.User, error) {
	var user types.User
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1"
	} else {
		placeholder = "?"
	}

	err := db.QueryRow(`
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE username = `+placeholder,
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
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1"
	} else {
		placeholder = "?"
	}

	err := db.QueryRow(`
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = `+placeholder,
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
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1"
	} else {
		placeholder = "?"
	}

	err := db.QueryRow(`
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = `+placeholder,
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

// UpdateUserPassword updates a user's password hash and updated_at timestamp
func (db *DB) UpdateUserPassword(userID int64, newPasswordHash string) error {
	now := time.Now()
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1, $2, $3"
	} else {
		placeholder = "?, ?, ?"
	}

	_, err := db.Exec(`
		UPDATE users 
		SET password_hash = `+placeholder[0:2]+`, 
		    updated_at = `+placeholder[4:5]+`
		WHERE id = `+placeholder[7:8],
		newPasswordHash,
		now,
		userID,
	)
	return err
}

// Service Management Functions

// GetServiceByInstanceID retrieves a service configuration by its instance ID
func (db *DB) GetServiceByInstanceID(instanceID string) (*models.ServiceConfiguration, error) {
	var service models.ServiceConfiguration
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1"
	} else {
		placeholder = "?"
	}

	err := db.QueryRow(`
		SELECT id, instance_id, display_name, url, api_key 
		FROM service_configurations 
		WHERE instance_id = `+placeholder, instanceID).Scan(
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

// GetServiceByURL retrieves a service configuration by its URL
func (db *DB) GetServiceByURL(url string) (*models.ServiceConfiguration, error) {
	var service models.ServiceConfiguration
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1"
	} else {
		placeholder = "?"
	}

	err := db.QueryRow(`
		SELECT id, instance_id, display_name, url, api_key 
		FROM service_configurations 
		WHERE url = `+placeholder, url).Scan(
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
	var query string
	if db.driver == "postgres" {
		query = `
			SELECT id, instance_id, display_name, url, api_key 
			FROM service_configurations 
			WHERE instance_id LIKE $1 || '%'
			LIMIT 1`
	} else {
		query = `
			SELECT id, instance_id, display_name, url, api_key 
			FROM service_configurations 
			WHERE instance_id LIKE ? || '%'
			LIMIT 1`
	}

	err := db.QueryRow(query, prefix).Scan(
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
	if db.driver == "postgres" {
		err := db.QueryRow(`
			INSERT INTO service_configurations (instance_id, display_name, url, api_key)
			VALUES ($1, $2, $3, $4)
			RETURNING id`,
			service.InstanceID,
			service.DisplayName,
			service.URL,
			service.APIKey,
		).Scan(&service.ID)
		return err
	}

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
	var query string
	if db.driver == "postgres" {
		query = `
			UPDATE service_configurations 
			SET display_name = $1, url = $2, api_key = $3
			WHERE instance_id = $4`
	} else {
		query = `
			UPDATE service_configurations 
			SET display_name = ?, url = ?, api_key = ?
			WHERE instance_id = ?`
	}

	_, err := db.Exec(query,
		service.DisplayName,
		service.URL,
		service.APIKey,
		service.InstanceID,
	)
	return err
}

// DeleteService deletes a service configuration by its instance ID
func (db *DB) DeleteService(instanceID string) error {
	var placeholder string
	if db.driver == "postgres" {
		placeholder = "$1"
	} else {
		placeholder = "?"
	}

	_, err := db.Exec(`
		DELETE FROM service_configurations 
		WHERE instance_id = `+placeholder,
		instanceID,
	)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
