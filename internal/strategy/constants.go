package strategy

import "time"

const (
	// Capital Management
	INITIAL_BALANCE = 12000.0
	LOT_SIZE        = 65
	MIN_TRADE_PRICE = 40.0

	// Transaction Charges
	FIXED_CHARGE_PER_TRADE = 40.0
	TURNOVER_CHARGE_PCT    = 0.0005

	// Strategy Defaults
	DEFAULT_SL_ENTRY_POINTS = 1
	DEFAULT_SL_EXIT_POINTS  = 1

	STREAM_TICK_RATE = 1/1000 * time.Millisecond // 0.01ms = 10 microseconds
	SLIPPAGE_TICKS   = 0
)
