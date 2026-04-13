package maker

import (
	"context"
	"math/big"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/ethclient"
	luxgeth "github.com/luxfi/geth"
)

// toCallMsg builds an ethereum.CallMsg from address + data.
func toCallMsg(to common.Address, data []byte) luxgeth.CallMsg {
	return luxgeth.CallMsg{
		To:   &to,
		Data: data,
	}
}

// callContract is a thin wrapper that re-dials per call (simple, correct).
func callContract(ctx context.Context, rpcURL string, to common.Address, data []byte) ([]byte, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return client.CallContract(ctx, toCallMsg(to, data), nil)
}

// sendTx would be used for actual order submission (fill via ATS).
// Left as a stub — the maker currently quotes only, execution goes through
// the exchange frontend or the LiquidityProtocol adapter.
func sendTx(_ context.Context, _ string, _ common.Address, _ []byte, _ *big.Int) (common.Hash, error) {
	return common.Hash{}, nil // stub
}
