package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// ConnectDB initializes the database connection.
func ConnectDB() error {
	var err error
	// DSN format: username:password@tcp(host:port)/dbname?parseTime=true
	dsn := "root:password@tcp(localhost:3306)/smart_api?parseTime=true"

	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Set connection pool parameters
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Verify the connection
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Successfully connected to the smart_api database.")
	return nil
}

// CloseDB closes the database connection.
func CloseDB() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			log.Printf("Error closing database: %v\\n", err)
		} else {
			log.Println("Database connection closed properly.")
		}
	}
}

// TickData represents a single row in the tickbytickdata table.
type TickData struct {
	ID        int
	Symbol    string
	Data      string
	Datex     string
	CreatedAt string
}

// StreamTickData fetches and streams data at an interval for one or more symbols.
// If symbols slice is empty, it streams all symbols for the given date.
func StreamTickData(symbols []string, datex string, rate time.Duration, handler func(TickData) error) error {
	var query string
	var args []interface{}

	if len(symbols) == 0 {
		query = `SELECT id, symbol, data, datex, created_at FROM tickbytickdata WHERE datex = ? ORDER BY id ASC`
		args = append(args, datex)
	} else if len(symbols) == 1 {
		query = `SELECT id, symbol, data, datex, created_at FROM tickbytickdata WHERE symbol = ? AND datex = ? ORDER BY id ASC`
		args = append(args, symbols[0], datex)
	} else {
		// Building IN (?, ?, ...)
		placeholders := ""
		for i := range symbols {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, symbols[i])
		}
		query = fmt.Sprintf(`SELECT id, symbol, data, datex, created_at FROM tickbytickdata WHERE symbol IN (%s) AND datex = ? ORDER BY id ASC`, placeholders)
		args = append(args, datex)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var tick TickData
		if err := rows.Scan(&tick.ID, &tick.Symbol, &tick.Data, &tick.Datex, &tick.CreatedAt); err != nil {
			return fmt.Errorf("row scan failed: %w", err)
		}

		if handler != nil {
			if err := handler(tick); err != nil {
				return fmt.Errorf("handler error: %w", err)
			}
		}

		time.Sleep(rate)
	}

	if count == 0 {
		fmt.Printf("No tick records found for symbols: %v on date: %s\n", symbols, datex)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	return nil
}

// GetUniqueSymbols returns a list of unique symbols for a given date.
func GetUniqueSymbols(date string) ([]string, error) {
	query := `SELECT DISTINCT symbol FROM tickbytickdata WHERE datex = ?`
	rows, err := DB.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		symbols = append(symbols, s)
	}
	return symbols, nil
}
