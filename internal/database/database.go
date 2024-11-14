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

		// Configure connection pool
		database.SetMaxOpenConns(25)
		database.SetMaxIdleConns(25)
		database.SetConnMaxLifetime(5 * time.Minute)
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

		// Set busy timeout
		if _, err = database.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
			return nil, errors.Wrap(err, "busy timeout pragma")
		}

		// Enable WAL. SQLite performs better with the WAL  because it allows
		// multiple readers to operate while data is being written.
		if _, err = database.Exec(`PRAGMA journal_mode = wal;`); err != nil {
			return nil, errors.Wrap(err, "enable wal")
		}

		// SQLite has a query planner that uses lifecycle stats to fund optimizations.
		// This restricts the SQLite query planner optimizer to only run if sufficient
		// information has been gathered over the lifecycle of the connection.
		// The SQLite documentation is inconsistent in this regard,
		// suggestions of 400 and 1000 are both "recommended", so lets use the lower bound.
		if _, err = database.Exec(`PRAGMA analysis_limit = 400;`); err != nil {
			return nil, errors.Wrap(err, "analysis_limit")
		}

		// When the application does not cleanly shut down, the WAL will still be present and not committed.
		// This is a no-op if the WAL is empty, and a commit when the WAL is not to start fresh.
		// When commits hit 1000, PRAGMA wal_checkpoint(PASSIVE); is invoked which tries its best
		// to commit from the WAL (and can fail to commit all pending operations).
		// Forcing a PRAGMA wal_checkpoint(RESTART); in the future on a "quiet period" could be
		// considered.
		if _, err = database.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
			return nil, errors.Wrap(err, "commit wal")
		}

		// Enable foreign key checks. For historical reasons, SQLite does not check
		// foreign key constraints by default. There's some overhead on inserts to
		// verify foreign key integrity, but it's definitely worth it.
		if _, err = database.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
			return nil, errors.New("foreign keys pragma")
		}

		// Now that the file exists, set restrictive permissions
		if err := os.Chmod(config.Path, 0640); err != nil {
			return nil, fmt.Errorf("error setting database file permissions: %w", err)
		}
		log.Debug().
			Str("path", config.Path).
			Msg("Initializing SQLite database")
	}

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
			api_key TEXT,
			access_url TEXT
		)`, autoIncrement))
	if err != nil {
		return err
	}

	// Add access_url column if it doesn't exist
	if db.driver == "postgres" {
		_, err = db.Exec(`
			DO $$ 
			BEGIN 
				BEGIN
					ALTER TABLE service_configurations ADD COLUMN access_url TEXT;
				EXCEPTION 
					WHEN duplicate_column THEN 
						NULL;
				END;
			END $$;
		`)
	} else {
		// For SQLite, check if column exists first
		var count int
		err = db.QueryRow(`
			SELECT COUNT(*) 
			FROM pragma_table_info('service_configurations') 
			WHERE name='access_url'
		`).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			_, err = db.Exec(`ALTER TABLE service_configurations ADD COLUMN access_url TEXT`)
		}
	}
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
func (db *DB) HasUsers(ctx context.Context) (bool, error) {
	qb := db.squirrel.Select("COUNT(*)").From("users")

	query, args, err := qb.ToSql()
	if err != nil {
		return false, err
	}

	var count int
	err = db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// User Management Functions

// CreateUser creates a new user in the database
func (db *DB) CreateUser(ctx context.Context, user *types.User) error {
	now := time.Now()

	queryBuilder := db.squirrel.Insert("users").
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

// FindUser retrieves a user by FindUserParams
func (db *DB) FindUser(ctx context.Context, params types.FindUserParams) (*types.User, error) {
	queryBuilder := db.squirrel.Select("id", "username", "email", "password_hash", "created_at", "updated_at").From("users")

	or := sq.Or{}

	if params.ID != 0 {
		or = append(or, sq.Eq{"id": params.ID})
	}
	if params.Username != "" {
		or = append(or, sq.Eq{"username": params.Username})
	}
	if params.Email != "" {
		or = append(or, sq.Eq{"email": params.Email})
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
func (db *DB) UpdateUserPassword(ctx context.Context, userID int64, newPasswordHash string) error {
	now := time.Now()

	queryBuilder := db.squirrel.Update("users").
		Set("password_hash", newPasswordHash).
		Set("updated_at", now).
		Where(sq.Eq{"id": userID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}

// Service Management Functions

// FindServiceBy retrieves a service configuration by FindServiceParams
func (db *DB) FindServiceBy(ctx context.Context, params types.FindServiceParams) (*models.ServiceConfiguration, error) {
	queryBuilder := db.squirrel.Select("id", "instance_id", "display_name", "url", "api_key", "access_url").
		From("service_configurations")

	if params.InstanceID != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"instance_id": params.InstanceID})
	}

	if params.InstancePrefix != "" {
		queryBuilder = queryBuilder.Where(sq.Like{"instance_id": params.InstancePrefix + "%"})
	}

	if params.URL != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"url": params.URL})
	}

	if params.AccessURL != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"access_url": params.AccessURL})
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var service models.ServiceConfiguration
	var url, apiKey, accessURL sql.NullString

	err = db.QueryRowContext(ctx, query, args...).Scan(
		&service.ID,
		&service.InstanceID,
		&service.DisplayName,
		&url,
		&apiKey,
		&accessURL,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Only set optional fields if they're not NULL
	if url.Valid {
		service.URL = url.String
	}
	if apiKey.Valid {
		service.APIKey = apiKey.String
	}
	if accessURL.Valid {
		service.AccessURL = accessURL.String
	}

	return &service, nil
}

// GetServiceByInstancePrefix retrieves a service configuration by its instance ID prefix
func (db *DB) GetServiceByInstancePrefix(ctx context.Context, prefix string) (*models.ServiceConfiguration, error) {
	var service models.ServiceConfiguration
	var url, apiKey, accessURL sql.NullString

	var query string
	if db.driver == "postgres" {
		query = `
			SELECT id, instance_id, display_name, url, api_key, access_url
			FROM service_configurations 
			WHERE instance_id LIKE $1 || '%'
			LIMIT 1`
	} else {
		query = `
			SELECT id, instance_id, display_name, url, api_key, access_url
			FROM service_configurations 
			WHERE instance_id LIKE ? || '%'
			LIMIT 1`
	}

	err := db.QueryRowContext(ctx, query, prefix).Scan(
		&service.ID,
		&service.InstanceID,
		&service.DisplayName,
		&url,
		&apiKey,
		&accessURL,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Only set optional fields if they're not NULL
	if url.Valid {
		service.URL = url.String
	}
	if apiKey.Valid {
		service.APIKey = apiKey.String
	}
	if accessURL.Valid {
		service.AccessURL = accessURL.String
	}

	return &service, nil
}

// GetAllServices retrieves all service configurations
func (db *DB) GetAllServices(ctx context.Context) ([]models.ServiceConfiguration, error) {
	queryBuilder := db.squirrel.Select("id", "instance_id", "display_name", "url", "api_key", "access_url").
		From("service_configurations")

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []models.ServiceConfiguration
	for rows.Next() {
		var service models.ServiceConfiguration
		var url, apiKey, accessURL sql.NullString

		err := rows.Scan(
			&service.ID,
			&service.InstanceID,
			&service.DisplayName,
			&url,
			&apiKey,
			&accessURL,
		)
		if err != nil {
			return nil, err
		}

		if url.Valid {
			service.URL = url.String
		}
		if apiKey.Valid {
			service.APIKey = apiKey.String
		}
		if accessURL.Valid {
			service.AccessURL = accessURL.String
		}

		services = append(services, service)
	}

	return services, nil
}

// CreateService creates a new service configuration
func (db *DB) CreateService(ctx context.Context, service *models.ServiceConfiguration) error {
	queryBuilder := db.squirrel.Insert("service_configurations").
		Columns("instance_id", "display_name", "url", "api_key", "access_url").
		Values(service.InstanceID, service.DisplayName, service.URL, service.APIKey, service.AccessURL).
		Suffix("RETURNING id").RunWith(db.DB)

	if err := queryBuilder.QueryRowContext(ctx).Scan(&service.ID); err != nil {
		return errors.Wrap(err, "error executing query")
	}

	return nil
}

// UpdateService updates an existing service configuration
func (db *DB) UpdateService(ctx context.Context, service *models.ServiceConfiguration) error {
	queryBuilder := db.squirrel.Update("service_configurations").
		Set("display_name", service.DisplayName).
		Set("url", sql.NullString{String: service.URL, Valid: service.URL != ""}).
		Set("api_key", sql.NullString{String: service.APIKey, Valid: service.APIKey != ""}).
		Set("access_url", sql.NullString{String: service.AccessURL, Valid: service.AccessURL != ""}).
		Where(sq.Eq{"instance_id": service.InstanceID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, query, args...)
	return err
}

// DeleteService deletes a service configuration by its instance ID
func (db *DB) DeleteService(ctx context.Context, instanceID string) error {
	queryBuilder := db.squirrel.Delete("service_configurations").Where(sq.Eq{"instance_id": instanceID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	res, err := db.ExecContext(ctx, query, args...)
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
