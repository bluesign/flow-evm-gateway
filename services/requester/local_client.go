package requester

import (
	"context"
	"fmt"
	"math/big"

	evmTypes "github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/go-ethereum/common"

	errs "github.com/onflow/flow-evm-gateway/models/errors"
	"github.com/onflow/flow-evm-gateway/services/state"
	"github.com/onflow/flow-evm-gateway/storage"
)

var _ EVMClient = &LocalClient{}

func NewLocalClient(state *state.BlockState, blocks storage.BlockIndexer) *LocalClient {
	return &LocalClient{
		state:  state,
		blocks: blocks,
	}
}

type LocalClient struct {
	state  *state.BlockState
	blocks storage.BlockIndexer
}

func (l *LocalClient) SendRawTransaction(
	ctx context.Context,
	data []byte,
) (common.Hash, error) {
	return common.Hash{}, fmt.Errorf("local client is read-only")
}

func (l *LocalClient) GetBalance(
	ctx context.Context,
	address common.Address,
	evmHeight int64,
) (*big.Int, error) {
	bal := l.state.GetBalance(address)
	return (&big.Int{}).SetUint64(bal.Uint64()), nil
}

func (l *LocalClient) Call(
	ctx context.Context,
	data []byte,
	from common.Address,
	evmHeight int64,
) ([]byte, error) {
	res, err := l.state.Call(from, data)
	if err != nil {
		return nil, err
	}

	result := res.ResultSummary()
	if result.ErrorCode != 0 {
		if result.ErrorCode == evmTypes.ExecutionErrCodeExecutionReverted {
			return nil, errs.NewRevertError(result.ReturnedData)
		}
		return nil, errs.NewFailedTransactionError(result.ErrorMessage)
	}

	return res.ReturnedData, nil
}

func (l *LocalClient) EstimateGas(
	ctx context.Context,
	data []byte,
	from common.Address,
	evmHeight int64,
) (uint64, error) {
	res, err := l.state.Call(from, data)
	if err != nil {
		return 0, err
	}
	return res.GasConsumed, nil
}

func (l *LocalClient) GetNonce(
	ctx context.Context,
	address common.Address,
	evmHeight int64,
) (uint64, error) {
	return l.state.GetNonce(address), nil
}

func (l *LocalClient) GetCode(
	ctx context.Context,
	address common.Address,
	evmHeight int64,
) ([]byte, error) {
	return l.state.GetCode(address), nil
}

func (l *LocalClient) GetStorageAt(
	ctx context.Context,
	address common.Address,
	hash common.Hash,
	evmHeight int64,
) (common.Hash, error) {
	return l.state.GetState(address, hash), nil
}

func (l *LocalClient) GetLatestEVMHeight(ctx context.Context) (uint64, error) {
	return l.blocks.LatestEVMHeight()
}