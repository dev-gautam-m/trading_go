package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	// "time"
	"smart_api_cli/internal/database"
	"smart_api_cli/internal/strategy"
	"smart_api_cli/internal/worker"
)

func main() {
	// Define basic CLI flags
	helpFlag := flag.Bool("help", false, "Show help information")
	pingFlag := flag.Bool("ping", false, "Ping the smart_api database to test connection")
	symbolFlag := flag.String("symbol", "NIFTY24MAR2622700CE", "Symbol to filter for stream")
	dateFlag := flag.String("date", "2026-03-24", "Date string (YYYY-MM-DD)")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A basic CLI application connected to smart_api DB.\\n\\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	// Always initialize the database connection for this application
	err := database.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// If the user specified ping, only show ping and exit.
	if *pingFlag {
		fmt.Println("Database connection is active and healthy.")
		return
	}

	// Default behavior: Start streaming and applying the strategy.
	symbolList := strings.Split(*symbolFlag, ",")
	var symbols []string
	for _, s := range symbolList {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			symbols = append(symbols, trimmed)
		}
	}

	// Initialize Multi-symbol Dispatcher (Queue + Workers)
	d := worker.NewDispatcher(1000, strategy.DEFAULT_SL_ENTRY_POINTS, strategy.DEFAULT_SL_EXIT_POINTS)
	d.Start()

	handler := func(tick database.TickData) error {
		d.Enqueue(tick)
		return nil
	}

	fmt.Printf("Starting Multi-symbol Strategy for %v on %s...\n", symbols, *dateFlag)
	streamRate := strategy.STREAM_TICK_RATE
	err = database.StreamTickData(symbols, *dateFlag, streamRate, handler)
	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}

	// Wait for processing to finish and print final stats
	d.Stop()
	fmt.Println("Strategy run finished.")
}
