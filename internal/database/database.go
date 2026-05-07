package database

import (
	"database/sql"
	"fmt"
	"os"

	// Blank import: we do not use this package directly in our code.
	// But importing it runs its init() function, which registers the
	// "sqlite3" driver with Go's database/sql package.
	// Without this line, sql.Open("sqlite3", ...) would fail.
	_ "github.com/mattn/go-sqlite3"
)

// Init opens the SQLite database file and initializes the schema.
// It returns a *sql.DB connection pool ready for use.
// The caller (main.go) is responsible for closing it with db.Close().
func Init(dbPath string, schemaPath string) (*sql.DB, error) {

	// sql.Open does not actually connect to the database yet.
	// It just validates the arguments and prepares the connection pool.
	// The first real connection happens on the first query.
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// db.Ping() forces an actual connection to the database.
	// This is where we find out if the file path is valid and accessible.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// SQLite does not enforce foreign keys by default.
	// We must enable it explicitly on every new connection.
	// Without this, ON DELETE CASCADE and FK constraints are silently ignored.
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Read the schema.sql file from disk.
	// os.ReadFile returns the entire file as a []byte slice.
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute the schema SQL.
	// db.Exec runs a SQL statement and discards the result rows.
	// Because every statement uses CREATE TABLE IF NOT EXISTS,
	// this is safe to run every time — it will not overwrite existing data.
	_, err = db.Exec(string(schema))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil

}
