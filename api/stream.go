package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/onflow/flow-evm-gateway/services/logs"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	errs "github.com/onflow/flow-evm-gateway/api/errors"
	"github.com/onflow/flow-evm-gateway/config"
	"github.com/onflow/flow-evm-gateway/storage"
	storageErrs "github.com/onflow/flow-evm-gateway/storage/errors"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	flowgoStorage "github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

// subscriptionBufferLimit is a constant that represents the buffer limit for subscriptions.
// It defines the maximum number of events that can be buffered in a subscription channel
const subscriptionBufferLimit = 1

type StreamAPI struct {
	logger                  zerolog.Logger
	config                  *config.Config
	blocks                  storage.BlockIndexer
	transactions            storage.TransactionIndexer
	receipts                storage.ReceiptIndexer
	blocksBroadcaster       *engine.Broadcaster
	transactionsBroadcaster *engine.Broadcaster
	logsBroadcaster         *engine.Broadcaster
}

func NewStreamAPI(
	logger zerolog.Logger,
	config *config.Config,
	blocks storage.BlockIndexer,
	transactions storage.TransactionIndexer,
	receipts storage.ReceiptIndexer,
	blocksBroadcaster *engine.Broadcaster,
	transactionsBroadcaster *engine.Broadcaster,
	logsBroadcaster *engine.Broadcaster,
) *StreamAPI {
	return &StreamAPI{
		logger:                  logger,
		config:                  config,
		blocks:                  blocks,
		transactions:            transactions,
		receipts:                receipts,
		blocksBroadcaster:       blocksBroadcaster,
		transactionsBroadcaster: transactionsBroadcaster,
		logsBroadcaster:         logsBroadcaster,
	}
}

// newSubscription creates a new subscription for receiving events from the broadcaster.
// The data adapter function is used to transform the raw data received from the broadcaster
// to comply with requested RPC API response schema.
func (s *StreamAPI) newSubscription(
	ctx context.Context,
	broadcaster *engine.Broadcaster,
	getData backend.GetDataByHeightFunc,
) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	height, err := s.blocks.LatestEVMHeight()
	if err != nil {
		return nil, err
	}
	height += 1 // subscribe to the next new event which will produce next height

	sub := backend.NewHeightBasedSubscription(subscriptionBufferLimit, height, getData)

	rpcSub := notifier.CreateSubscription()
	rpcSub.ID = rpc.ID(sub.ID()) // make sure ids are unified

	l := s.logger.With().Str("subscription-id", string(rpcSub.ID)).Logger()
	l.Info().Uint64("evm-height", height).Msg("new subscription created")

	go backend.NewStreamer(
		s.logger.With().Str("component", "streamer").Logger(),
		broadcaster,
		s.config.StreamTimeout,
		s.config.StreamLimit,
		sub,
	).Stream(context.Background()) // todo investigate why the passed in context is canceled so quickly

	go func() {
		for {
			select {
			case data, open := <-sub.Channel():
				if !open {
					l.Debug().Msg("subscription channel closed")
					return
				}

				l.Debug().Str("subscription-id", string(rpcSub.ID)).Any("data", data).Msg("notifying new event")
				err = notifier.Notify(rpcSub.ID, data)
				if err != nil {
					l.Err(err).Msg("failed to notify")
				}
			case err := <-rpcSub.Err():
				l.Error().Err(err).Msg("error from rpc subscriber")
				sub.Close()
				return
			case <-notifier.Closed():
				sub.Close()
				return
			}
		}
	}()

	return rpcSub, nil
}

// NewHeads send a notification each time a new block is appended to the chain.
func (s *StreamAPI) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	sub, err := s.newSubscription(
		ctx,
		s.blocksBroadcaster,
		func(ctx context.Context, height uint64) (interface{}, error) {
			block, err := s.blocks.GetByHeight(height)
			if err != nil {
				fmt.Println("# new blocks [block] not found", height)
				if errors.Is(err, storageErrs.ErrNotFound) { // make sure to wrap in not found error as the streamer expects it
					return nil, errors.Join(flowgoStorage.ErrNotFound, err)
				}
				return nil, fmt.Errorf("failed to get block at height: %d: %w", height, err)
			}

			h, err := block.Hash()
			if err != nil {
				return nil, errs.ErrInternal
			}
			// todo there is a lot of missing data: https://docs.chainstack.com/reference/ethereum-native-subscribe-newheads
			return Block{
				Hash:         h,
				Number:       hexutil.Uint64(block.Height),
				ParentHash:   block.ParentBlockHash,
				ReceiptsRoot: block.ReceiptRoot,
				Transactions: block.TransactionHashes}, nil
		},
	)
	l := s.logger.With().Str("subscription-id", string(sub.ID)).Logger()
	if err != nil {
		l.Error().Err(err).Msg("error creating a new heads subscription")
		return nil, err
	}

	l.Info().Msg("new heads subscription created")
	return sub, nil
}

// NewPendingTransactions creates a subscription that is triggered each time a
// transaction enters the transaction pool. If fullTx is true the full tx is
// sent to the client, otherwise the hash is sent.
func (s *StreamAPI) NewPendingTransactions(ctx context.Context, fullTx *bool) (*rpc.Subscription, error) {
	sub, err := s.newSubscription(
		ctx,
		s.transactionsBroadcaster,
		func(ctx context.Context, height uint64) (interface{}, error) {
			block, err := s.blocks.GetByHeight(height)
			if err != nil {
				fmt.Println("# pending tx [block] not found", height)
				if errors.Is(err, storageErrs.ErrNotFound) { // make sure to wrap in not found error as the streamer expects it
					return nil, errors.Join(flowgoStorage.ErrNotFound, err)
				}
				return nil, fmt.Errorf("failed to get block at height: %d: %w", height, err)
			}

			// todo once a block can contain multiple transactions this needs to be refactored
			if len(block.TransactionHashes) != 1 {
				return nil, fmt.Errorf("block contains more than a single transaction")
			}
			hash := block.TransactionHashes[0]

			tx, err := s.transactions.Get(hash)
			if err != nil {
				fmt.Println("# pending tx a [transaction] not found", height)
				if errors.Is(err, storageErrs.ErrNotFound) { // make sure to wrap in not found error as the streamer expects it
					return nil, errors.Join(flowgoStorage.ErrNotFound, err)
				}
				return nil, fmt.Errorf("failed to get tx with hash: %s at height: %d: %w", hash, height, err)
			}

			rcp, err := s.receipts.GetByTransactionID(hash)
			if err != nil {
				fmt.Println("# pending tx [receipt] not found", height)
				if errors.Is(err, storageErrs.ErrNotFound) { // make sure to wrap in not found error as the streamer expects it
					return nil, errors.Join(flowgoStorage.ErrNotFound, err)
				}
				return nil, fmt.Errorf("failed to get receipt with hash: %s at height: %d: %w", hash, height, err)
			}

			h, err := tx.Hash()
			if fullTx != nil && *fullTx == false || fullTx == nil { // make sure not nil
				return h, nil
			}
			return NewTransaction(tx, *rcp)
		},
	)
	l := s.logger.With().Str("subscription-id", string(sub.ID)).Logger()
	if err != nil {
		l.Error().Err(err).Msg("error creating a new pending transactions subscription")
		return nil, err
	}

	l.Info().Msg("new pending transactions subscription created")
	return sub, nil
}

// Logs creates a subscription that fires for all new log that match the given filter criteria.
func (s *StreamAPI) Logs(ctx context.Context, criteria filters.FilterCriteria) (*rpc.Subscription, error) {
	if len(criteria.Topics) > maxTopics {
		return nil, errExceedMaxTopics
	}
	if len(criteria.Addresses) > maxAddresses {
		return nil, errExceedMaxAddresses
	}

	sub, err := s.newSubscription(
		ctx,
		s.logsBroadcaster,
		func(ctx context.Context, height uint64) (interface{}, error) {
			block, err := s.blocks.GetByHeight(height)
			if err != nil {
				return nil, fmt.Errorf("failed to get block at height: %d: %w", height, err)
			}

			id, err := block.Hash()
			if err != nil {
				return nil, err
			}

			// convert from the API type
			f := logs.FilterCriteria{
				Addresses: criteria.Addresses,
				Topics:    criteria.Topics,
			}
			return logs.NewIDFilter(id, f, s.blocks, s.receipts).Match()
		},
	)
	l := s.logger.With().Str("subscription-id", string(sub.ID)).Logger()
	if err != nil {
		l.Error().Err(err).Msg("failed creating logs subscription")
		return nil, err
	}

	l.Error().Err(err).Msg("new logs subscription created")
	return sub, nil
}
