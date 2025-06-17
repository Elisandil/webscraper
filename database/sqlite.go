package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	*sql.DB
}

func NewSQLiteDB(dbPath string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	sqliteDB := &SQLiteDB{DB: db}
	if err := sqliteDB.createTables(); err != nil {
		return nil, err
	}

	return sqliteDB, nil
}

func (db *SQLiteDB) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS scraping_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		title TEXT,
		description TEXT,
		keywords TEXT,
		author TEXT,
		language TEXT,
		favicon TEXT,
		image_url TEXT,
		site_name TEXT,
		links TEXT,
		images TEXT,
		headers TEXT,
		status_code INTEGER,
		content_type TEXT,
		word_count INTEGER DEFAULT 0,
		load_time_ms INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_scraping_results_url ON scraping_results(url);
	CREATE INDEX IF NOT EXISTS idx_scraping_results_created_at ON scraping_results(created_at);
	CREATE INDEX IF NOT EXISTS idx_scraping_results_status_code ON scraping_results(status_code);`

	_, err := db.Exec(query)
	return err
}
