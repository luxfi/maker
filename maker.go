package maker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Maker is the market making engine. It periodically aggregates external
// orderbook depth, calculates quotes via a strategy, and tracks position.
type Maker struct {
	cfg        Config
	aggregator *Aggregator
	strategy   Strategy
	position   float64
	mu         sync.Mutex
	orders     map[string][]Order // active orders by symbol
	logger     *slog.Logger
	stopCh     chan struct{}
	stopped    chan struct{}
}

// New creates a market maker.
func New(cfg Config, aggregator *Aggregator, strategy Strategy, logger *slog.Logger) *Maker {
	return &Maker{
		cfg:        cfg,
		aggregator: aggregator,
		strategy:   strategy,
		orders:     make(map[string][]Order),
		logger:     logger,
		stopCh:     make(chan struct{}),
		stopped:    make(chan struct{}),
	}
}

// Start runs the quote loop at RefreshInterval. Blocks until ctx is cancelled
// or Stop is called.
func (m *Maker) Start(ctx context.Context) error {
	m.logger.Info("starting maker", "symbols", m.cfg.Symbols, "spread_bps", m.cfg.SpreadBps, "levels", m.cfg.Levels, "interval", m.cfg.RefreshInterval)

	if len(m.cfg.Symbols) == 0 {
		return fmt.Errorf("no symbols configured")
	}

	defer close(m.stopped)

	ticker := time.NewTicker(m.cfg.RefreshInterval)
	defer ticker.Stop()

	// Initial quote immediately.
	m.requoteAll(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("maker stopped", "reason", "context cancelled")
			return ctx.Err()
		case <-m.stopCh:
			m.logger.Info("maker stopped", "reason", "stop called")
			return nil
		case <-ticker.C:
			m.requoteAll(ctx)
		}
	}
}

// Stop signals the maker to shut down gracefully.
func (m *Maker) Stop() {
	select {
	case <-m.stopCh:
		// Already stopped.
	default:
		close(m.stopCh)
	}
	<-m.stopped
}

// OnFill updates position tracking when an order is filled.
func (m *Maker) OnFill(symbol, side string, qty, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch side {
	case "buy":
		m.position += qty
	case "sell":
		m.position -= qty
	}

	m.logger.Info("fill", "symbol", symbol, "side", side, "qty", qty, "price", price, "position", m.position)
}

// GetUnifiedBook returns the aggregated book including phantom depth from
// external venues.
func (m *Maker) GetUnifiedBook(ctx context.Context, symbol string) (*AggregatedBook, error) {
	return m.aggregator.GetAggregatedBook(ctx, symbol)
}

// Position returns the current net position.
func (m *Maker) Position() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.position
}

// ActiveOrders returns the last set of quotes for a symbol.
func (m *Maker) ActiveOrders(symbol string) []Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders[symbol]
}

func (m *Maker) requoteAll(ctx context.Context) {
	for _, sym := range m.cfg.Symbols {
		if err := m.requote(ctx, sym); err != nil {
			m.logger.Error("requote failed", "symbol", sym, "error", err)
		}
	}
}

func (m *Maker) requote(ctx context.Context, symbol string) error {
	book, err := m.aggregator.GetAggregatedBook(ctx, symbol)
	if err != nil {
		return fmt.Errorf("get book: %w", err)
	}

	m.mu.Lock()
	pos := m.position
	m.mu.Unlock()

	// Check position limits — skip the overweight side.
	skipBids := pos >= m.cfg.MaxPositionQty
	skipAsks := pos <= -m.cfg.MaxPositionQty

	bids, asks, err := m.strategy.Quote(ctx, book, pos, m.cfg)
	if err != nil {
		return fmt.Errorf("strategy quote: %w", err)
	}

	if skipBids {
		bids = nil
	}
	if skipAsks {
		asks = nil
	}

	// Store active orders.
	var all []Order
	all = append(all, bids...)
	all = append(all, asks...)

	m.mu.Lock()
	m.orders[symbol] = all
	m.mu.Unlock()

	m.logger.Info("requoted", "symbol", symbol, "mid", book.MidPrice,
		"bids", len(bids), "asks", len(asks), "sources", book.Sources, "position", pos)
	return nil
}
