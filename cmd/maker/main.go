package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/luxfi/maker"
)

func main() {
	broker := flag.String("broker", "http://localhost:8090", "Broker API base URL")
	symbols := flag.String("symbols", "BTC-USD", "Comma-separated symbols to market make")
	spread := flag.Float64("spread", 10, "Half-spread in basis points")
	levels := flag.Int("levels", 5, "Price levels per side")
	interval := flag.Duration("interval", 5*time.Second, "Requote interval")
	skew := flag.Float64("skew", 2, "Inventory skew in basis points")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	cfg := maker.DefaultConfig()
	cfg.BrokerURL = *broker
	cfg.Symbols = strings.Split(*symbols, ",")
	cfg.SpreadBps = *spread
	cfg.Levels = *levels
	cfg.RefreshInterval = *interval
	cfg.SkewBps = *skew

	// In production the consumer wires real broker HTTP calls here.
	// This stub provider returns synthetic data for testing.
	stub := maker.ProviderSource{
		Name: "stub",
		GetBBO: func(_ context.Context, _ string) (float64, float64, float64, error) {
			mid := 50000.0 + rand.Float64()*100 - 50
			return mid - 5, mid + 5, mid, nil
		},
		GetBook: func(_ context.Context, _ string) ([]maker.PriceLevel, []maker.PriceLevel, error) {
			mid := 50000.0 + rand.Float64()*100 - 50
			bids := make([]maker.PriceLevel, 5)
			asks := make([]maker.PriceLevel, 5)
			for i := range 5 {
				offset := float64(i+1) * 5
				bids[i] = maker.PriceLevel{Price: mid - offset, Qty: float64(i+1) * 0.5, Source: "stub", Phantom: true}
				asks[i] = maker.PriceLevel{Price: mid + offset, Qty: float64(i+1) * 0.5, Source: "stub", Phantom: true}
			}
			return bids, asks, nil
		},
	}

	aggregator := maker.NewAggregator([]maker.ProviderSource{stub}, logger)
	strategy := &maker.InventorySkewStrategy{}
	m := maker.New(cfg, aggregator, strategy, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := m.Start(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "maker exited: %v\n", err)
		os.Exit(1)
	}
}
