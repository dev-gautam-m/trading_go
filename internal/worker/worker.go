package worker

import (
	"fmt"
	"strconv"
	"sync"
	"smart_api_cli/internal/database"
	"smart_api_cli/internal/strategy"
)

type workerState struct {
	strategy      *strategy.Strategy
	initialized   bool
	lastPrice     float64
	lastTimestamp string
}

// Dispatcher manages multiple strategies and routes ticks to them.
type Dispatcher struct {
	states     map[string]*workerState
	mu         sync.RWMutex
	tickChan   chan database.TickData
	wg         sync.WaitGroup

	// Strategy Params
	slEntryPoints float64
	slExitPoints  float64
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(bufferSize int, slEntry, slExit float64) *Dispatcher {
	return &Dispatcher{
		states:        make(map[string]*workerState),
		tickChan:      make(chan database.TickData, bufferSize),
		slEntryPoints: slEntry,
		slExitPoints:  slExit,
	}
}

// Start starts the dispatcher worker.
func (d *Dispatcher) Start() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for tick := range d.tickChan {
			d.processTick(tick)
		}
	}()
}

// Stop stops the dispatcher and waits for it to finish.
func (d *Dispatcher) Stop() {
	close(d.tickChan)
	d.wg.Wait()
	d.PrintAllStats()
}

// Enqueue adds a tick to the queue.
func (d *Dispatcher) Enqueue(tick database.TickData) {
	d.tickChan <- tick
}

// processTick routes the tick to the correct strategy.
func (d *Dispatcher) processTick(tick database.TickData) {
	d.mu.Lock()
	state, ok := d.states[tick.Symbol]
	if !ok {
		fmt.Printf("Initializing new strategy for symbol: %s\n", tick.Symbol)
		state = &workerState{
			strategy: strategy.NewStrategy(tick.Symbol, d.slEntryPoints, d.slExitPoints),
		}
		d.states[tick.Symbol] = state
	}
	d.mu.Unlock()

	// Parse price
	priceRaw, err := strconv.ParseFloat(tick.Data, 64)
	if err != nil {
		fmt.Printf("Error parsing price for %s: %v\n", tick.Symbol, err)
		return
	}
	price := priceRaw / 100.0

	if !state.initialized {
		state.strategy.Init(price, tick.CreatedAt)
		state.initialized = true
	} else {
		state.strategy.OnPrice(price, tick.CreatedAt)
	}

	state.lastPrice = price
	state.lastTimestamp = tick.CreatedAt
}

// PrintAllStats prints statistics for all active strategies.
func (d *Dispatcher) PrintAllStats() {
	fmt.Printf("\n================ ALL SYMBOLS FINAL STATISTICS =================\n")
	d.mu.RLock()
	defer d.mu.RUnlock()

	for sym, state := range d.states {
		fmt.Printf("--- %s ---\n", sym)
		state.strategy.ForceExit(state.lastPrice, state.lastTimestamp)
		state.strategy.PrintStats()
	}
	fmt.Printf("===============================================================\n")
}

// GetStrategyFor returns the strategy instance for a given symbol.
func (d *Dispatcher) GetStrategyFor(symbol string) *strategy.Strategy {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if state, ok := d.states[symbol]; ok {
		return state.strategy
	}
	return nil
}
