package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
)

// Event represents a row from the events table.
type Event struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Action       string    `json:"action"`
	MetadataPage *string   `json:"metadata_page,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Eventter interface {
	// InsertEvent inserts a new event and returns the created event id.
	InsertEvent(ctx context.Context, userID int64, action string, metadata map[string]string) (int64, error)
	// GetEvents returns events filtered by optional userID, start and end timestamps.
	GetEvents(ctx context.Context, userID *int64, start *time.Time, end *time.Time) ([]Event, error)
}

type Aggregatter interface {
	// AggregateEvents aggregates events into user_event_counts for the provided period length (seconds).
	AggregateEvents(seconds int) error
}

// Service represents a service that interacts with a database.
type Service interface {
	// Health returns a map of health status information.
	// The keys and values in the map are service-specific.
	Health() map[string]string

	// Close terminates the database connection.
	// It returns an error if the connection cannot be closed.
	Close() error

	Eventter

	Aggregatter
}

type service struct {
	db *sql.DB
}

var (
	database   = os.Getenv("DB_DATABASE")
	password   = os.Getenv("DB_PASSWORD")
	username   = os.Getenv("DB_USERNAME")
	port       = os.Getenv("DB_PORT")
	host       = os.Getenv("DB_HOST")
	schema     = os.Getenv("DB_SCHEMA")
	dbInstance *service
)

func New() Service {
	// Reuse Connection
	if dbInstance != nil {
		return dbInstance
	}
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s", username, password, host, port, database, schema)
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	dbInstance = &service{
		db: db,
	}
	return dbInstance
}

// Health checks the health of the database connection by pinging the database.
// It returns a map with keys indicating various health statistics.
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	// Ping the database
	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Fatalf("db down: %v", err) // Log the error and terminate the program
		return stats
	}

	// Database is up, add more statistics
	stats["status"] = "up"
	stats["message"] = "It's healthy"

	// Get database stats (like open connections, in use, idle, etc.)
	dbStats := s.db.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	// Evaluate stats to provide a health message
	if dbStats.OpenConnections > 40 { // Assuming 50 is the max for this example
		stats["message"] = "The database is experiencing heavy load."
	}

	if dbStats.WaitCount > 1000 {
		stats["message"] = "The database has a high number of wait events, indicating potential bottlenecks."
	}

	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings."
	}

	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern."
	}

	return stats
}

// Close closes the database connection.
// It logs a message indicating the disconnection from the specific database.
// If the connection is successfully closed, it returns nil.
// If an error occurs while closing the connection, it returns the error.
func (s *service) Close() error {
	log.Printf("Disconnected from database: %s", database)
	return s.db.Close()
}

// InsertEvent inserts a new event into the events table.
// metadata is stored in the metadata_page column as plain text or JSON string depending on input.
func (s *service) InsertEvent(ctx context.Context, userID int64, action string, metadata map[string]string) (int64, error) {
	// For now we'll store metadata.page into metadata_page column if present.
	var metadataPage sql.NullString
	if metadata != nil {
		if page, ok := metadata["page"]; ok {
			metadataPage = sql.NullString{String: page, Valid: true}
		}
	}

	query := `INSERT INTO events(user_id, action, metadata_page) VALUES ($1, $2, $3) RETURNING id`
	var id int64
	// Use QueryRowContext to return the inserted id
	err := s.db.QueryRowContext(ctx, query, userID, action, metadataPage).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetEvents queries events table using optional filters.
// Uses the provided SQL:
// SELECT id, user_id, action, metadata_page, created_at
// FROM events
// WHERE ($1::bigint IS NULL OR user_id = $1)
// AND ($2::timestamptz IS NULL OR created_at >= $2)
// AND ($3::timestamptz IS NULL OR created_at <= $3)
// ORDER BY created_at DESC;
func (s *service) GetEvents(ctx context.Context, userID *int64, start *time.Time, end *time.Time) ([]Event, error) {
	query := `
SELECT id, user_id, action, metadata_page, created_at
FROM events
WHERE ($1::bigint IS NULL OR user_id = $1)
AND ($2::timestamptz IS NULL OR created_at >= $2)
AND ($3::timestamptz IS NULL OR created_at <= $3)
ORDER BY created_at DESC;
`
	var uid interface{} = nil
	if userID != nil {
		uid = *userID
	}
	var startVal interface{} = nil
	if start != nil {
		startVal = *start
	}
	var endVal interface{} = nil
	if end != nil {
		endVal = *end
	}

	rows, err := s.db.QueryContext(ctx, query, uid, startVal, endVal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var e Event
		var metadata sql.NullString
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		if metadata.Valid {
			e.MetadataPage = &metadata.String
		} else {
			e.MetadataPage = nil
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// AggregateEvents creates/upserts aggregated counts into user_event_counts for the time window defined
// by nowUTC - seconds .. nowUTC. It uses an INSERT ... ON CONFLICT to upsert per (user_id, period_start).
func (s *service) AggregateEvents(seconds int) error {
	periodEnd := time.Now().UTC()
	periodStart := periodEnd.Add(-time.Duration(seconds) * time.Second)

	_, err := s.db.Exec(`
	INSERT INTO user_event_counts (user_id, period_start, period_end, event_count)
	SELECT user_id, $1, $2, COUNT(*) FROM events
	WHERE created_at >= $1 AND created_at < $2
	GROUP BY user_id
	ON CONFLICT (user_id, period_start)
	DO UPDATE SET event_count = EXCLUDED.event_count;
	`, periodStart, periodEnd)
	if err == sql.ErrNoRows {
		return nil
	}

	return err
}
