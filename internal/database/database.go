// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

// DB represents the database connection
type DB struct {
	*sql.DB
	driver string
	path   string

	squirrel sq.StatementBuilderType
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

	db := &DB{
		DB:     database,
		driver: config.Driver,
		path:   config.Path,
		// set default placeholder for squirrel to support both sqlite and postgres
		squirrel: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("error initializing schema: %w", err)
	}

	return db, nil
}

// Path returns the database file path (for SQLite)
func (db *DB) Path() string {
	return db.path
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
	qb := sq.Select("COUNT(*)").From("users")

	query, args, err := qb.ToSql()
	if err != nil {
		return false, err
	}

	var count int
	err = db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// User Management Functions

// CreateUser creates a new user in the database
func (db *DB) CreateUser(user *types.User) error {
	ctx := context.Background()

	now := time.Now()

	queryBuilder := sq.Insert("users").
		Columns("username", "email", "password_hash", "created_at", "updated_at").
		Values(user.Username, user.Email, user.PasswordHash, now, now).
		Suffix("RETURNING id").RunWith(db.DB)

	if err := queryBuilder.QueryRowContext(ctx).Scan(&user.ID); err != nil {
		return errors.Wrap(err, "error executing query")
	}

	user.CreatedAt = now
	user.UpdatedAt = now

	return nil
}

type FindUserParams struct {
	ID       int64
	Username string
	Email    string
}

// FindUser retrieves a user by FindUserParams
func (db *DB) FindUser(ctx context.Context, params FindUserParams) (*types.User, error) {
	queryBuilder := sq.Select("id", "username", "email", "password_hash", "created_at", "updated_at").From("users")

	or := sq.Or{}

	if params.ID != 0 {
		or = append(or, sq.Eq{"id": params.ID})
		//queryBuilder = queryBuilder.Where(sq.Eq{"ud": params.ID})
	}
	if params.Username != "" {
		or = append(or, sq.Eq{"username": params.Username})
		//queryBuilder = queryBuilder.Where(sq.Eq{"username": params.Username})
	}
	if params.Email != "" {
		or = append(or, sq.Eq{"email": params.Email})
		//queryBuilder = queryBuilder.Where(sq.Eq{"email": params.Email})
	}

	queryBuilder = queryBuilder.Where(or)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var user types.User
	err = db.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

// UpdateUserPassword updates a user's password hash and updated_at timestamp
func (db *DB) UpdateUserPassword(userID int64, newPasswordHash string) error {
	now := time.Now()

	queryBuilder := sq.Update("users").
		Set("password_hash", newPasswordHash).
		Set("updated_at", now).
		Where(sq.Eq{"id": userID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(context.Background(), query, args...)
	if err != nil {
		return err
	}

	return nil
}

// Service Management Functions

type FindServiceParams struct {
	InstanceID     string
	InstancePrefix string
	URL            string
}

// FindServiceBy retrieves a service configuration by FindServiceParams
func (db *DB) FindServiceBy(ctx context.Context, params FindServiceParams) (*models.ServiceConfiguration, error) {
	queryBuilder := sq.Select("id", "instance_id", "display_name", "url", "api_key").From("service_configurations")

	if params.InstanceID != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"instance_id": params.InstanceID})
	}

	if params.InstancePrefix != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"instance_id": params.InstancePrefix + "%"})
	}

	if params.URL != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"url": params.URL})
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	row := db.QueryRowContext(ctx, query, args...)
	if row.Err() != nil {
		return nil, row.Err()
	}

	var service models.ServiceConfiguration
	if err := row.Scan(&service.ID, &service.InstanceID, &service.DisplayName, &service.URL, &service.APIKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

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
	queryBuilder := sq.Select("id", "instance_id", "display_name", "url", "api_key").From("service_configurations")

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(context.Background(), query, args...)
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
	ctx := context.Background()

	queryBuilder := sq.Insert("service_configurations").
		Columns("instance_id", "display_name", "url", "api_key").
		Values(service.InstanceID, service.DisplayName, service.URL, service.APIKey).
		Suffix("RETURNING id").RunWith(db.DB)

	if err := queryBuilder.QueryRowContext(ctx).Scan(&service.ID); err != nil {
		return errors.Wrap(err, "error executing query")
	}

	return nil
}

// UpdateService updates an existing service configuration
func (db *DB) UpdateService(service *models.ServiceConfiguration) error {
	queryBuilder := sq.Update("service_configurations").
		Set("display_name", service.DisplayName).
		Set("url", service.URL).
		Set("api_key", service.APIKey).
		Where(sq.Eq{"instance_id": service.InstanceID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(context.Background(), query, args...)
	if err != nil {
		return err
	}

	return nil
}

// DeleteService deletes a service configuration by its instance ID
func (db *DB) DeleteService(instanceID string) error {
	queryBuilder := sq.Delete("service_configurations").Where(sq.Eq{"instance_id": instanceID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	res, err := db.ExecContext(context.Background(), query, args...)
	if err != nil {
		return err
	}

	if rowsAffected, err := res.RowsAffected(); err != nil {
		if rowsAffected == 0 {
			return nil
		}
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
