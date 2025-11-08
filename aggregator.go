package maker

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

// PriceLevel is a single price/quantity entry in an orderbook.
type PriceLevel struct {
	Price   float64
	Qty     float64
	Source  string // "native" for CEX CLOB, provider name for external
	Phantom bool   // true if from external venue (display only, not fillable)
}

// AggregatedBook is merged orderbook depth from all venues.
type AggregatedBook struct {
	Symbol   string
	Bids     []PriceLevel // sorted highest first
	Asks     []PriceLevel // sorted lowest first
	MidPrice float64
	Sources  []string
}

// ProviderSource defines how to fetch data from a single venue.
type ProviderSource struct {
	Name    string
	GetBBO  func(ctx context.Context, symbol string) (bid, ask, last float64, err error)
	GetBook func(ctx context.Context, symbol string) (bids, asks []PriceLevel, err error)
}

// Aggregator merges orderbooks from multiple venues.
type Aggregator struct {
	providers []ProviderSource
	logger    *slog.Logger
}

// NewAggregator creates an aggregator from the given provider sources.
func NewAggregator(sources []ProviderSource, logger *slog.Logger) *Aggregator {
	return &Aggregator{
		providers: sources,
		logger:    logger,
	}
}

// GetAggregatedBook fetches from all sources in parallel, merges, and sorts.
func (a *Aggregator) GetAggregatedBook(ctx context.Context, symbol string) (*AggregatedBook, error) {
	type result struct {
		name string
		bids []PriceLevel
		asks []PriceLevel
		err  error
	}

	results := make([]result, len(a.providers))
	var wg sync.WaitGroup
	for i, p := range a.providers {
		wg.Add(1)
		go func(idx int, prov ProviderSource) {
			defer wg.Done()
			bids, asks, err := prov.GetBook(ctx, symbol)
			results[idx] = result{name: prov.Name, bids: bids, asks: asks, err: err}
		}(i, p)
	}
	wg.Wait()

	var allBids, allAsks []PriceLevel
	var sources []string
	var errs int

	for _, r := range results {
		if r.err != nil {
			a.logger.Warn("provider fetch failed", "provider", r.name, "error", r.err)
			errs++
			continue
		}
		sources = append(sources, r.name)
		allBids = append(allBids, r.bids...)
		allAsks = append(allAsks, r.asks...)
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("all %d providers failed for %s", errs, symbol)
	}

	// Sort bids descending by price.
	sort.Slice(allBids, func(i, j int) bool {
		return allBids[i].Price > allBids[j].Price
	})
	// Sort asks ascending by price.
	sort.Slice(allAsks, func(i, j int) bool {
		return allAsks[i].Price < allAsks[j].Price
	})

	var mid float64
	if len(allBids) > 0 && len(allAsks) > 0 {
		mid = (allBids[0].Price + allAsks[0].Price) / 2
	}

	return &AggregatedBook{
		Symbol:   symbol,
		Bids:     allBids,
		Asks:     allAsks,
		MidPrice: mid,
		Sources:  sources,
	}, nil
}

// GetMidPrice returns the mid from the aggregated book.
func (a *Aggregator) GetMidPrice(ctx context.Context, symbol string) (float64, error) {
	book, err := a.GetAggregatedBook(ctx, symbol)
	if err != nil {
		return 0, err
	}
	if book.MidPrice <= 0 {
		return 0, fmt.Errorf("no valid mid for %s", symbol)
	}
	return book.MidPrice, nil
}
