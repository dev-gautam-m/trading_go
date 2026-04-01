package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"smart_api_cli/internal/database"
	"smart_api_cli/internal/strategy"
	"smart_api_cli/internal/worker"
)

func main() {
	// Connect to database
	err := database.ConnectDB()
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	// Serve static files
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// API Endpoints
	http.HandleFunc("/api/symbols", handleSymbols)
	http.HandleFunc("/api/simulate", handleSimulate)

	port := ":8085"
	fmt.Printf("Server starting on http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleSymbols(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "date parameter is required", http.StatusBadRequest)
		return
	}

	symbols, err := database.GetUniqueSymbols(date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(symbols)
}

type SimulateRequest struct {
	Date    string   `json:"date"`
	Symbols []string `json:"symbols"`
}

type SimulationResult struct {
	Symbol   string             `json:"symbol"`
	Summary  map[string]interface{} `json:"summary"`
	Trades   []strategy.Trade   `json:"trades"`
}

func handleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Initialize Multi-symbol Dispatcher
	d := worker.NewDispatcher(1000, strategy.DEFAULT_SL_ENTRY_POINTS, strategy.DEFAULT_SL_EXIT_POINTS)
	d.Start()

	handler := func(tick database.TickData) error {
		d.Enqueue(tick)
		return nil
	}

	// Run simulation (same logic as CLI)
	err := database.StreamTickData(req.Symbols, req.Date, strategy.STREAM_TICK_RATE, handler)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Collect results
	d.Stop() // This also prints to console, but we need the internal data

	results := []SimulationResult{}
	for _, sym := range req.Symbols {
		// Note: We need a way to get the strategy results back from the worker.
		// Let's assume d.GetResultsFor(sym) exists or we access d.States directly.
		// I'll update worker.go to allow this.
		strat := d.GetStrategyFor(sym)
		if strat != nil {
			netPnL := strat.Balance - strat.InitialBalance
			results = append(results, SimulationResult{
				Symbol: sym,
				Summary: map[string]interface{}{
					"initial_capital": strat.InitialBalance,
					"final_balance":   strat.Balance,
					"net_pnl":         netPnL,
					"net_pnl_pct":     (netPnL / strat.InitialBalance) * 100,
					"total_charges":   strat.TotalCharges,
					"total_trades":    strat.TotalTrades,
					"win_trades":      strat.WinTrades,
					"loss_trades":     strat.LossTrades,
					"win_points":      strat.WinPoints,
					"loss_points":     strat.LossPoints,
				},
				Trades: strat.Trades,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
