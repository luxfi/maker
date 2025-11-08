package maker

import "time"

// Config controls market maker behavior.
type Config struct {
	// BrokerURL is the broker API base URL.
	BrokerURL string

	// Symbols to make markets on.
	Symbols []string

	// SpreadBps is the half-spread in basis points (10 = 10bps each side).
	SpreadBps float64

	// Levels is how many price levels to quote on each side.
	Levels int

	// LevelSpacingBps is spacing between levels in basis points.
	LevelSpacingBps float64

	// BaseQty is the quantity at the best level.
	BaseQty float64

	// QtyMultiplier increases qty at deeper levels (1.5 = 50% more per level).
	QtyMultiplier float64

	// RefreshInterval is how often to requote.
	RefreshInterval time.Duration

	// Providers to aggregate depth from (empty = all registered).
	Providers []string

	// PhantomDepth includes external venue depth in unified book view.
	PhantomDepth bool

	// MaxPositionQty caps net position before stopping one-sided quoting.
	MaxPositionQty float64

	// SkewBps adjusts spread based on inventory.
	SkewBps float64
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		SpreadBps:       10,
		Levels:          5,
		LevelSpacingBps: 5,
		BaseQty:         1.0,
		QtyMultiplier:   1.5,
		RefreshInterval: 5 * time.Second,
		PhantomDepth:    true,
		MaxPositionQty:  100,
		SkewBps:         2,
	}
}
