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

	"github.com/autobrr/dashbrr/internal/database/migrations"
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/types"
)

// DB represents the database connection
type DB struct {
	*sql.DB
	driver string
	path   string
	config *Config

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
	db := &DB{
		driver: config.Driver,
		path:   config.Path,
		config: config,
		// set default placeholder for squirrel to support both sqlite and postgres
		squirrel: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	if err := db.Open(); err != nil {
		return nil, err
	}

	log.Info().
		Str("driver", config.Driver).
		Msg("Successfully connected to database")

	return db, nil
}

// Path returns the database file path (for SQLite)
func (db *DB) Path() string {
	return db.path
}

func (db *DB) Open() error {
	switch db.driver {
	case "postgres":
		if err := db.openPostgres(); err != nil {
			return err
		}

	case "sqlite":
		if err := db.openSQLite(); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported database driver: %s", db.driver)
	}

	return nil
}

func (db *DB) openSQLite() error {
	// SQLite connection
	dbDir := filepath.Dir(db.config.Path)

	// Create directory with restricted permissions
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		return err
	}

	// Create or open database
	database, err := sql.Open("sqlite", db.config.Path)
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}

	// Force SQLite to create the database file by pinging it
	if err := database.Ping(); err != nil {
		return fmt.Errorf("error creating database file: %w", err)
	}

	// Set busy timeout
	if _, err = database.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return errors.Wrap(err, "busy timeout pragma")
	}

	// Enable WAL. SQLite performs better with the WAL  because it allows
	// multiple readers to operate while data is being written.
	if _, err = database.Exec(`PRAGMA journal_mode = wal;`); err != nil {
		return errors.Wrap(err, "enable wal")
	}

	// SQLite has a query planner that uses lifecycle stats to fund optimizations.
	// This restricts the SQLite query planner optimizer to only run if sufficient
	// information has been gathered over the lifecycle of the connection.
	// The SQLite documentation is inconsistent in this regard,
	// suggestions of 400 and 1000 are both "recommended", so lets use the lower bound.
	if _, err = database.Exec(`PRAGMA analysis_limit = 400;`); err != nil {
		return errors.Wrap(err, "analysis_limit")
	}

	// When the application does not cleanly shut down, the WAL will still be present and not committed.
	// This is a no-op if the WAL is empty, and a commit when the WAL is not to start fresh.
	// When commits hit 1000, PRAGMA wal_checkpoint(PASSIVE); is invoked which tries its best
	// to commit from the WAL (and can fail to commit all pending operations).
	// Forcing a PRAGMA wal_checkpoint(RESTART); in the future on a "quiet period" could be
	// considered.
	if _, err = database.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
		return errors.Wrap(err, "commit wal")
	}

	// Enable foreign key checks. For historical reasons, SQLite does not check
	// foreign key constraints by default. There's some overhead on inserts to
	// verify foreign key integrity, but it's definitely worth it.
	if _, err = database.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return errors.New("foreign keys pragma")
	}

	db.DB = database

	// Now that the file exists, set restrictive permissions
	if err := os.Chmod(db.config.Path, 0640); err != nil {
		return fmt.Errorf("error setting database file permissions: %w", err)
	}

	log.Debug().
		Str("path", db.config.Path).
		Msg("Initializing SQLite database")

	if err := migrations.SQLiteMigrator(db.DB); err != nil {
		return errors.Wrap(err, "error running sqlite migrations")
	}

	return nil
}

func (db *DB) openPostgres() error {
	maxRetries := 5
	baseDelay := time.Second

	// PostgreSQL connection
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		db.config.Host, db.config.Port, db.config.User, db.config.Password, db.config.DBName)

	log.Debug().
		Str("host", db.config.Host).
		Str("port", db.config.Port).
		Str("database", db.config.DBName).
		Msg("Initializing PostgreSQL database")

	var database *sql.DB
	var err error

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
			return fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
		}

		delay := time.Duration(attempt) * baseDelay
		log.Debug().
			Int("attempt", attempt).
			Dur("delay", delay).
			Msg("Retrying database connection")

		time.Sleep(delay)
	}

	db.DB = database

	// Configure connection pool
	db.DB.SetMaxOpenConns(25)
	db.DB.SetMaxIdleConns(25)
	db.DB.SetConnMaxLifetime(5 * time.Minute)

	log.Debug().Msg("Initializing PostgreSQL database")

	if err := migrations.PostgresMigrator(db.DB); err != nil {
		return errors.Wrap(err, "error running postgres migrations")
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
func (db *DB) HasUsers() (bool, error) {
	qb := db.squirrel.Select("COUNT(*)").From("users")

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

	queryBuilder := db.squirrel.Update("users").
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
func (db *DB) GetServiceByInstancePrefix(prefix string) (*models.ServiceConfiguration, error) {
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

	err := db.QueryRow(query, prefix).Scan(
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
func (db *DB) GetAllServices() ([]models.ServiceConfiguration, error) {
	queryBuilder := db.squirrel.Select("id", "instance_id", "display_name", "url", "api_key", "access_url").
		From("service_configurations")

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
func (db *DB) CreateService(service *models.ServiceConfiguration) error {
	ctx := context.Background()

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
func (db *DB) UpdateService(service *models.ServiceConfiguration) error {
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

	_, err = db.ExecContext(context.Background(), query, args...)
	return err
}

// DeleteService deletes a service configuration by its instance ID
func (db *DB) DeleteService(instanceID string) error {
	queryBuilder := db.squirrel.Delete("service_configurations").Where(sq.Eq{"instance_id": instanceID})

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
