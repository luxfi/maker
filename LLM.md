# maker — AI Assistant Context

# maker

Automated market maker engine for Lux Network exchanges. Aggregates orderbook depth from multiple external venues, calculates a fair mid price, and generates limit order quotes on both sides.

## Module

`github.com/luxfi/maker` — Go library + CLI.

## Key Types

- `Maker` — quote loop (start/stop, position tracking, requote)
- `Aggregator` — merges orderbooks from N `ProviderSource` in parallel
- `Strategy` — interface: `MidSpreadStrategy` (symmetric), `InventorySkewStrategy` (position-aware)
- `OMAProviderSource` — reads on-chain OracleMirroredAMM prices via EVM RPC
- `Config` — spread, levels, qty, refresh interval, phantom depth, position limits

## Usage

```
go run ./cmd/maker -symbols BTC-USD,ETH-USD -spread 10 -levels 5 -interval 5s
```

Wire real venues by implementing `ProviderSource` (see README.md).
