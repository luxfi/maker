package maker

import (
	"context"
	"fmt"
	"math/big"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/ethclient"
)

// OMA ABI selectors (first 4 bytes of keccak256).
var (
	// getExecutionPrice(string,bool) → (uint256,uint256)
	sigGetExecPrice = common.Hex2Bytes("e7572230") // computed from ABI
	// swap(string,bool,uint256,uint256) → uint256
	sigSwap = common.Hex2Bytes("6c5f5560")
)

// OMAProviderSource creates a ProviderSource that reads prices from the
// OracleMirroredAMM contract on any EVM chain. This feeds the maker's
// aggregator so it can quote around the oracle mid.
func OMAProviderSource(name, rpcURL, omaAddr string) ProviderSource {
	return ProviderSource{
		Name: name,
		GetBBO: func(ctx context.Context, symbol string) (bid, ask, last float64, err error) {
			client, err := ethclient.DialContext(ctx, rpcURL)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("oma dial: %w", err)
			}
			defer client.Close()

			addr := common.HexToAddress(omaAddr)

			buyPrice, err := callGetExecPrice(ctx, client, addr, symbol, true)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("oma buy price: %w", err)
			}
			sellPrice, err := callGetExecPrice(ctx, client, addr, symbol, false)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("oma sell price: %w", err)
			}

			askF := weiToFloat(buyPrice)
			bidF := weiToFloat(sellPrice)
			mid := (askF + bidF) / 2
			return bidF, askF, mid, nil
		},
		GetBook: func(ctx context.Context, symbol string) (bids, asks []PriceLevel, err error) {
			// OMA has no orderbook depth — it's oracle-priced with infinite
			// depth at the execution price. Return a single level per side.
			client, err := ethclient.DialContext(ctx, rpcURL)
			if err != nil {
				return nil, nil, fmt.Errorf("oma dial: %w", err)
			}
			defer client.Close()

			addr := common.HexToAddress(omaAddr)
			buyP, _ := callGetExecPrice(ctx, client, addr, symbol, true)
			sellP, _ := callGetExecPrice(ctx, client, addr, symbol, false)

			// Infinite depth represented as a large phantom qty.
			const phantomQty = 1_000_000.0
			if buyP != nil && buyP.Sign() > 0 {
				asks = []PriceLevel{{Price: weiToFloat(buyP), Qty: phantomQty, Source: name, Phantom: true}}
			}
			if sellP != nil && sellP.Sign() > 0 {
				bids = []PriceLevel{{Price: weiToFloat(sellP), Qty: phantomQty, Source: name, Phantom: true}}
			}
			return bids, asks, nil
		},
	}
}

// callGetExecPrice encodes and calls getExecutionPrice(string,bool).
func callGetExecPrice(ctx context.Context, client *ethclient.Client, addr common.Address, symbol string, isBuy bool) (*big.Int, error) {
	// ABI encode: getExecutionPrice(string,bool)
	// Manual encoding to avoid importing abigen-generated code.
	symbolBytes := []byte(symbol)
	// offset for string = 64, bool at 32, string length at 64, data at 96+
	data := make([]byte, 0, 4+32+32+32+len(symbolBytes)+32)
	data = append(data, sigGetExecPrice...)
	// offset to string data = 0x40 (64)
	data = append(data, leftPad32(big.NewInt(64).Bytes())...)
	// bool isBuy
	if isBuy {
		data = append(data, leftPad32(big.NewInt(1).Bytes())...)
	} else {
		data = append(data, leftPad32(big.NewInt(0).Bytes())...)
	}
	// string length
	data = append(data, leftPad32(big.NewInt(int64(len(symbolBytes))).Bytes())...)
	// string data padded to 32 bytes
	padded := make([]byte, ((len(symbolBytes)+31)/32)*32)
	copy(padded, symbolBytes)
	data = append(data, padded...)

	msg := map[string]interface{}{
		"to":   addr,
		"data": fmt.Sprintf("0x%x", data),
	}
	_ = msg

	// Use eth_call via the client
	result, err := client.CallContract(ctx, toCallMsg(addr, data), nil)
	if err != nil {
		return nil, err
	}
	if len(result) < 32 {
		return nil, fmt.Errorf("short result")
	}
	return new(big.Int).SetBytes(result[:32]), nil
}

func weiToFloat(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}
	f := new(big.Float).SetInt(wei)
	f.Quo(f, new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
	result, _ := f.Float64()
	return result
}

func leftPad32(b []byte) []byte {
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return padded
}
