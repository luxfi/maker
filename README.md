# maker

Automated market maker engine for Lux Network exchanges. Aggregates orderbook depth from multiple external venues, calculates a fair mid price, and generates limit order quotes on both sides.

## Usage

```
go run ./cmd/maker -symbols BTC-USD,ETH-USD -spread 10 -levels 5 -interval 5s
```

## Integration

The maker is designed as a library. Wire real venue data by implementing `ProviderSource`:

```go
source := maker.ProviderSource{
    Name: "binance",
    GetBBO: func(ctx context.Context, symbol string) (bid, ask, last float64, err error) {
        // fetch from broker API
    },
    GetBook: func(ctx context.Context, symbol string) (bids, asks []maker.PriceLevel, err error) {
        // fetch from broker API
    },
}

agg := maker.NewAggregator([]maker.ProviderSource{source}, logger)
m := maker.New(cfg, agg, &maker.InventorySkewStrategy{}, logger)
m.Start(ctx)
```

## Strategies

- **MidSpreadStrategy** - symmetric quotes around aggregated mid
- **InventorySkewStrategy** - shifts mid based on inventory to reduce risk
