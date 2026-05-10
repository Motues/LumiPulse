package sqlite

import (
	"lumipluse-backend/internal/repository"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type repo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) repository.Repository {
	return &repo{db: db}
}

func InitSchema(db *sqlx.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS Service (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		url TEXT NOT NULL,
		type TEXT DEFAULT 'http',
		interval INTEGER DEFAULT 60,
		status TEXT DEFAULT 'operational',
		is_active INTEGER DEFAULT 1,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS Heartbeat (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		status INTEGER NOT NULL,
		latency INTEGER,
		message TEXT DEFAULT '',
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_heartbeat_service_time ON Heartbeat(service_id, created_at);

	CREATE TABLE IF NOT EXISTS ServiceDaily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		date TEXT NOT NULL,
		uptime_count INTEGER DEFAULT 0,
		downtime_count INTEGER DEFAULT 0,
		total_latency INTEGER DEFAULT 0,
		UNIQUE(service_id, date)
	);
	CREATE INDEX IF NOT EXISTS idx_daily_service_date ON ServiceDaily(service_id, date);

	CREATE TABLE IF NOT EXISTS Incident (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		impact TEXT NOT NULL,
		status TEXT DEFAULT 'investigating',
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_incident_status ON Incident(status);

	CREATE TABLE IF NOT EXISTS Incident_Update (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		incident_id INTEGER NOT NULL REFERENCES Incident(id) ON DELETE CASCADE,
		status TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_incident_update_incident ON Incident_Update(incident_id);

	CREATE TABLE IF NOT EXISTS Maintenance (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		scheduled_start DATETIME NOT NULL,
		scheduled_end DATETIME NOT NULL,
		status TEXT DEFAULT 'scheduled',
		affected_services TEXT DEFAULT '',
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_maintenance_status ON Maintenance(status);

	CREATE TABLE IF NOT EXISTS Settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS ApiKey (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		key TEXT NOT NULL UNIQUE,
		key_prefix TEXT NOT NULL,
		expires_at TEXT DEFAULT '',
		last_used_at TEXT DEFAULT '',
		last_used_ip TEXT DEFAULT '',
		is_active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_apikey_key ON ApiKey(key);`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Migrations for existing tables
	migrations := []string{
		`ALTER TABLE ServiceDaily ADD COLUMN total_latency INTEGER DEFAULT 0`,
	}
	for _, m := range migrations {
		db.Exec(m) // ignore errors (column may already exist)
	}

	return nil
}
