package maker

import (
	"context"
	"fmt"
	"math"
)

// Order is a limit order to be placed.
type Order struct {
	Side  string  // "buy" or "sell"
	Price float64
	Qty   float64
}

// Strategy determines order placement from aggregated book state.
type Strategy interface {
	Quote(ctx context.Context, book *AggregatedBook, position float64, cfg Config) (bids, asks []Order, err error)
}

// MidSpreadStrategy places symmetric quotes around the aggregated mid price.
type MidSpreadStrategy struct{}

func (s *MidSpreadStrategy) Quote(_ context.Context, book *AggregatedBook, _ float64, cfg Config) ([]Order, []Order, error) {
	if book.MidPrice <= 0 {
		return nil, nil, fmt.Errorf("invalid mid price: %f", book.MidPrice)
	}
	return generateLevels(book.MidPrice, 0, cfg)
}

// InventorySkewStrategy adjusts the mid based on current position to reduce
// inventory risk. When long, the mid shifts down (tighter asks, wider bids)
// to encourage selling. When short, the opposite.
type InventorySkewStrategy struct{}

func (s *InventorySkewStrategy) Quote(_ context.Context, book *AggregatedBook, position float64, cfg Config) ([]Order, []Order, error) {
	if book.MidPrice <= 0 {
		return nil, nil, fmt.Errorf("invalid mid price: %f", book.MidPrice)
	}

	// Skew ratio in [-1, 1], clamped.
	skewRatio := position / cfg.MaxPositionQty
	skewRatio = math.Max(-1, math.Min(1, skewRatio))

	// Shift mid away from accumulated side.
	skewAmount := book.MidPrice * (cfg.SkewBps / 10000) * skewRatio
	return generateLevels(book.MidPrice, skewAmount, cfg)
}

// generateLevels builds bid/ask order slices from the given mid and skew.
func generateLevels(mid, skew float64, cfg Config) ([]Order, []Order, error) {
	adjustedMid := mid - skew

	bids := make([]Order, 0, cfg.Levels)
	asks := make([]Order, 0, cfg.Levels)

	for i := range cfg.Levels {
		// Level 0 uses SpreadBps, deeper levels add LevelSpacingBps each.
		totalBps := cfg.SpreadBps + cfg.LevelSpacingBps*float64(i)
		offset := adjustedMid * (totalBps / 10000)
		qty := cfg.BaseQty * math.Pow(cfg.QtyMultiplier, float64(i))

		bidPrice := adjustedMid - offset
		askPrice := adjustedMid + offset

		if bidPrice > 0 {
			bids = append(bids, Order{
				Side:  "buy",
				Price: math.Round(bidPrice*1e8) / 1e8,
				Qty:   math.Round(qty*1e8) / 1e8,
			})
		}
		asks = append(asks, Order{
			Side:  "sell",
			Price: math.Round(askPrice*1e8) / 1e8,
			Qty:   math.Round(qty*1e8) / 1e8,
		})
	}

	return bids, asks, nil
}
