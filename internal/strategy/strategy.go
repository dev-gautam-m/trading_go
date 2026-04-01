package strategy

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Strategy represents the trading strategy state and statistics.
type Strategy struct {
	SL_ENTRY_POINTS float64 `json:"sl_entry_points"`
	SL_EXIT_POINTS  float64 `json:"sl_exit_points"`

	InTrade        bool    `json:"in_trade"`
	EntryPrice     float64 `json:"entry_price"`
	StopLoss       float64 `json:"stop_loss"`
	LastTrailLevel float64 `json:"last_trail_level"`
	EntryTrigger   float64 `json:"entry_trigger"`
	Symbol         string  `json:"symbol"`
	Balance        float64 `json:"balance"`
	InitialBalance float64 `json:"initial_balance"`
	LotSize        int     `json:"lot_size"`
	Quantity       int     `json:"quantity"`

	// Statistics
	TotalTrades  int     `json:"total_trades"`
	WinTrades    int     `json:"win_trades"`
	LossTrades   int     `json:"loss_trades"`
	WinPoints    float64 `json:"win_points"`
	LossPoints   float64 `json:"loss_points"`
	TotalCharges float64 `json:"total_charges"`
	Trades       []Trade `json:"trades"`
	IsStopped    bool    `json:"is_stopped"`

	// Slippage Management
	PendingAction     string `json:"-"`
	SlippageTickCount int    `json:"-"`
}

// Trade represents a single executed trade log.
type Trade struct {
	Number     int     `json:"number"`
	Type       string  `json:"type"`
	Time       string  `json:"time"`
	EntryPrice float64 `json:"entry_price"`
	ExitPrice  float64 `json:"exit_price"`
	Lots       int     `json:"lots"`
	Charges    float64 `json:"charges"`
	NetPnL     float64 `json:"net_pnl"`
	Balance    float64 `json:"balance"`
	Result     string  `json:"result"`
}

// NewStrategy creates a new strategy instance with given parameters.
func NewStrategy(symbol string, entryPoints, exitPoints float64) *Strategy {
	return &Strategy{
		Symbol:          symbol,
		SL_ENTRY_POINTS: entryPoints,
		SL_EXIT_POINTS:  exitPoints,
		InitialBalance:  INITIAL_BALANCE,
		Balance:         INITIAL_BALANCE,
		LotSize:         LOT_SIZE,
	}
}

// Init sets the initial entry trigger based on the current price.
func (s *Strategy) Init(currentPrice float64, timestamp string) {
	s.EntryTrigger = currentPrice + s.SL_ENTRY_POINTS
	placeBuyStopOrder(s.EntryTrigger)
}

// OnPrice handles a new price tick and executes strategy logic.
func (s *Strategy) OnPrice(price float64, timestamp string) {
	// 0. Check if instrument is stopped
	if s.IsStopped {
		return
	}

	// 1. Handle Pending Slippage Executions
	if s.PendingAction != "" {
		if s.SlippageTickCount >= SLIPPAGE_TICKS {
			if s.PendingAction == "ENTRY" {
				s.executeEntry(price, timestamp)
			} else if s.PendingAction == "EXIT" {
				s.executeExit(price, price, timestamp)
			}
			s.PendingAction = ""
			s.SlippageTickCount = 0
			return
		}
		s.SlippageTickCount++
		return
	}

	// 2. Proactive Price Cutoff
	if !s.InTrade && price < MIN_TRADE_PRICE {
		s.IsStopped = true
		return
	}

	// 3. Trailing Entry (Downward Trail)
	if !s.InTrade && (price+s.SL_ENTRY_POINTS < s.EntryTrigger) {
		s.EntryTrigger = price + s.SL_ENTRY_POINTS
		modifyBuyStopOrder(s.EntryTrigger)
	}

	// 4. Entry Signal Check
	if !s.InTrade && price >= s.EntryTrigger {
		if SLIPPAGE_TICKS == 0 {
			s.executeEntry(price, timestamp)
		} else {
			s.PendingAction = "ENTRY"
			s.SlippageTickCount = 1
		}
		return
	}

	// 5. Trailing Stop Loss
	if s.InTrade {
		trailed := false
		for price >= s.LastTrailLevel+s.SL_ENTRY_POINTS {
			s.StopLoss += s.SL_EXIT_POINTS
			s.LastTrailLevel += s.SL_ENTRY_POINTS
			trailed = true
		}
		if trailed {
			modifyStopLossOrder(s.StopLoss)
		}
	}

	// 6. Exit Signal Check
	if s.InTrade && price <= s.StopLoss {
		if SLIPPAGE_TICKS == 0 {
			s.executeExit(s.StopLoss, price, timestamp)
		} else {
			s.PendingAction = "EXIT"
			s.SlippageTickCount = 1
		}
		return
	}
}

// executeEntry performs the actual trade entry logic.
func (s *Strategy) executeEntry(price float64, timestamp string) {
	// Calculate Max Lots and account for estimated fees (Fixed + Turnover-based)
	denom := float64(s.LotSize) * (price + (2*price-s.SL_EXIT_POINTS)*TURNOVER_CHARGE_PCT)
	lots := int((s.Balance - FIXED_CHARGE_PER_TRADE) / denom)

	if lots < 1 {
		return
	}

	s.InTrade = true
	s.EntryPrice = price
	s.Quantity = lots * s.LotSize

	// Deduct premium
	cost := s.EntryPrice * float64(s.Quantity)
	s.Balance -= cost

	s.StopLoss = s.EntryPrice - s.SL_EXIT_POINTS
	s.LastTrailLevel = s.EntryPrice

	placeStopLossOrder(s.StopLoss)
}

// executeExit performs the actual trade exit logic.
func (s *Strategy) executeExit(execPrice float64, currentPrice float64, timestamp string) {
	s.TotalTrades++

	// Calculate proceeds and charges
	sellAmount := execPrice * float64(s.Quantity)
	turnover := (s.EntryPrice + execPrice) * float64(s.Quantity)
	charges := FIXED_CHARGE_PER_TRADE + (turnover * TURNOVER_CHARGE_PCT)

	s.Balance += (sellAmount - charges)
	s.TotalCharges += charges

	profitPoints := execPrice - s.EntryPrice
	result := "LOSS"
	if profitPoints >= 0 {
		s.WinTrades++
		s.WinPoints += profitPoints
		result = "WIN"
	} else {
		s.LossTrades++
		s.LossPoints += (-profitPoints)
	}

	// Format Timestamp
	formattedTime := timestamp
	if ms, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		formattedTime = time.UnixMilli(ms).Format("15:04:05")
	}

	// Record Trade
	tradeType := "RE-ENTRY"
	if s.TotalTrades == 1 {
		tradeType = "BUY"
	}

	s.Trades = append(s.Trades, Trade{
		Number:     s.TotalTrades,
		Type:       tradeType,
		Time:       formattedTime,
		EntryPrice: s.EntryPrice,
		ExitPrice:  execPrice,
		Lots:       s.Quantity / s.LotSize,
		Charges:    charges,
		NetPnL:     (sellAmount - charges) - (s.EntryPrice * float64(s.Quantity)),
		Balance:    s.Balance,
		Result:     result,
	})

	closePosition()

	// reset
	s.InTrade = false
	s.EntryPrice = 0
	s.StopLoss = 0
	s.LastTrailLevel = 0
	s.Quantity = 0

	// re-arm
	s.EntryTrigger = currentPrice + s.SL_ENTRY_POINTS
	placeBuyStopOrder(s.EntryTrigger)
}

// ForceExit closes any open position at the final known price.
func (s *Strategy) ForceExit(lastPrice float64, timestamp string) {
	if !s.InTrade {
		return
	}

	s.TotalTrades++

	// Calculate proceeds and charges (Market Exit at lastPrice)
	sellAmount := lastPrice * float64(s.Quantity)
	turnover := (s.EntryPrice + lastPrice) * float64(s.Quantity)
	charges := FIXED_CHARGE_PER_TRADE + (turnover * TURNOVER_CHARGE_PCT)

	s.Balance += (sellAmount - charges)
	s.TotalCharges += charges

	profitPoints := lastPrice - s.EntryPrice
	result := "LOSS"
	if profitPoints >= 0 {
		s.WinTrades++
		s.WinPoints += profitPoints
		result = "WIN"
	} else {
		s.LossTrades++
		s.LossPoints += (-profitPoints)
	}

	// Format Timestamp (re-use logic or just use timestamp)
	formattedTime := timestamp
	if ms, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		formattedTime = time.UnixMilli(ms).Format("15:04:05")
	}

	s.Trades = append(s.Trades, Trade{
		Number:     s.TotalTrades,
		Type:       "FORCE-EXIT",
		Time:       formattedTime,
		EntryPrice: s.EntryPrice,
		ExitPrice:  lastPrice,
		Lots:       s.Quantity / s.LotSize,
		Charges:    charges,
		NetPnL:     (sellAmount - charges) - (s.EntryPrice * float64(s.Quantity)),
		Balance:    s.Balance,
		Result:     result,
	})

	closePosition()
	s.InTrade = false
}

// PrintStats displays the detailed trade log table and financial summary.
func (s *Strategy) PrintStats() {
	fmt.Printf("\n================ TRADE LOG: %s ================\n", s.Symbol)
	fmt.Printf("%-4s %-10s %-20s %-10s %-10s %-5s %-10s %-12s %-12s %-6s\n",
		"#", "TYPE", "TIME", "ENTRY", "EXIT", "LOTS", "CHARGES", "NET PNL", "BALANCE", "RESULT")
	fmt.Println(strings.Repeat("-", 110))

	for _, t := range s.Trades {
		fmt.Printf("#%-3d %-10s %-20s %-10.2f %-10.2f %-5d %-10.2f %-12.2f %-12.2f %-6s\n",
			t.Number, t.Type, t.Time, t.EntryPrice, t.ExitPrice, t.Lots, t.Charges, t.NetPnL, t.Balance, t.Result)
	}
	fmt.Println(strings.Repeat("-", 110))

	netPnL := s.Balance - s.InitialBalance
	fmt.Printf("\n--- FINAL SUMMARY [%s] ---\n", s.Symbol)
	fmt.Printf("Initial Capital: ₹%.2f\n", s.InitialBalance)
	fmt.Printf("Final Balance:   ₹%.2f\n", s.Balance)
	fmt.Printf("Net Profit/Loss: ₹%.2f (%.2f%%)\n", netPnL, (netPnL/s.InitialBalance)*100)
	fmt.Printf("Total Charges:   ₹%.2f\n", s.TotalCharges)
	fmt.Printf("Total Trades:    %d (Win: %d, Loss: %d)\n", s.TotalTrades, s.WinTrades, s.LossTrades)
	fmt.Printf("Points:          Win: %.2f, Loss: %.2f\n", s.WinPoints, s.LossPoints)
	fmt.Printf("--------------------------------------------\n\n")
}

// waitForInput pauses execution until the user presses Enter.
func waitForInput() {
	fmt.Println(">>> Press ENTER to continue...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// Mock Order Functions (Now with pause after execution)

func placeBuyStopOrder(price float64) {
	// fmt.Printf(">>> Broker API: Placing BUY STOP order at %.2f\n", price)
	// waitForInput()
}

func modifyBuyStopOrder(price float64) {
	// fmt.Printf(">>> Broker API: Modifying BUY STOP order to %.2f\n", price)
	// waitForInput()
}

func placeStopLossOrder(price float64) {
	// fmt.Printf(">>> Broker API: Placing SELL STOP LOSS order at %.2f\n", price)
	// waitForInput()
}

func modifyStopLossOrder(price float64) {
	// fmt.Printf(">>> Broker API: Modifying STOP LOSS order to %.2f\n", price)
	// waitForInput()
}

func closePosition() {
	// fmt.Printf(">>> Broker API: Closing Position (Market Exit)\n")
	// waitForInput()
}
