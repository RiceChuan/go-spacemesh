package tests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	pb "github.com/spacemeshos/api/release/go/spacemesh/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/genvm/sdk"
	"github.com/spacemeshos/go-spacemesh/genvm/sdk/wallet"
	"github.com/spacemeshos/go-spacemesh/systest/chaos"
	"github.com/spacemeshos/go-spacemesh/systest/cluster"
	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

const (
	attempts = 3
)

var retryBackoff = 10 * time.Second

func sendTransactions(
	ctx context.Context,
	logger *zap.Logger,
	cl *cluster.Cluster,
	first, stop uint32,
	receiver types.Address,
	batch, amount int,
) error {
	eg, ctx := errgroup.WithContext(ctx)
	for i := range cl.Accounts() {
		client := cl.Client(i % cl.Total())
		nonce, err := getNonce(ctx, client, cl.Address(i))
		if err != nil {
			return fmt.Errorf("get nonce failed (%s: %s): %w", client.Name, cl.Address(i), err)
		}
		watchLayers(ctx, eg, client, logger, func(layer *pb.LayerStreamResponse) (bool, error) {
			if layer.Layer.Number.Number >= stop {
				return false, nil
			}
			if layer.Layer.Status != pb.Layer_LAYER_STATUS_APPROVED || layer.Layer.Number.Number < first {
				return true, nil
			}
			// give some time for a previous layer to be applied
			// TODO(dshulyak) introduce api that simply subscribes to internal clock
			// and outputs events when the tick for the layer is available
			time.Sleep(200 * time.Millisecond)
			if nonce == 0 {
				logger.Info("address needs to be spawned",
					zap.String("client", client.Name),
					zap.Stringer("address", cl.Address(i)),
				)
				if err := submitSpawn(ctx, cl, i, client); err != nil {
					return false, fmt.Errorf("failed to spawn %w", err)
				}
				nonce++
				return true, nil
			}
			logger.Debug("submitting transactions",
				zap.Uint32("layer", layer.Layer.Number.Number),
				zap.String("client", client.Name),
				zap.Stringer("address", cl.Address(i)),
				zap.Uint64("nonce", nonce),
				zap.Int("batch", batch),
			)
			for j := range batch {
				var err error
				for range 3 { // retry on failure 3 times
					err = submitSpend(ctx, cl, i, receiver, uint64(amount), nonce+uint64(j), client)
					if err == nil {
						break
					}
					logger.Warn("failed to spend",
						zap.String("client", client.Name),
						zap.Stringer("address", cl.Address(i)),
						zap.Uint64("nonce", nonce+uint64(j)),
						zap.Error(err),
					)
					time.Sleep(1 * time.Second) // wait before retrying
				}
				if err != nil {
					return false, fmt.Errorf("spend failed %s %w", client.Name, err)
				}
			}
			nonce += uint64(batch)
			return true, nil
		})
	}
	return eg.Wait()
}

func submitTransaction(ctx context.Context, tx []byte, node *cluster.NodeClient) ([]byte, error) {
	txclient := pb.NewTransactionServiceClient(node.PubConn())
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := txclient.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{Transaction: tx})
	if err != nil {
		return nil, err
	}
	if response.Txstate == nil {
		return nil, errors.New("tx state should not be nil")
	}
	return response.Txstate.Id.Id, nil
}

func stateHashStream(
	ctx context.Context,
	node *cluster.NodeClient,
	logger *zap.Logger,
	collector func(*pb.GlobalStateStreamResponse) (bool, error),
) error {
	retries := 0
BACKOFF:
	stateapi := pb.NewGlobalStateServiceClient(node.PubConn())
	states, err := stateapi.GlobalStateStream(ctx, &pb.GlobalStateStreamRequest{
		GlobalStateDataFlags: uint32(pb.GlobalStateDataFlag_GLOBAL_STATE_DATA_FLAG_GLOBAL_STATE_HASH),
	})
	if err != nil {
		return err
	}
	for {
		state, err := states.Recv()
		s, ok := status.FromError(err)
		if !ok {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("unknown error: %w", err)
		}
		switch s.Code() {
		case codes.OK:
			if cont, err := collector(state); !cont {
				return err
			}
		case codes.Canceled, codes.DeadlineExceeded:
			return nil
		case codes.Unavailable:
			if retries == attempts {
				return errors.New("state stream unavailable")
			}
			retries++
			time.Sleep(retryBackoff)
			goto BACKOFF
		default:
			logger.Warn(
				"global state stream error",
				zap.String("client", node.Name),
				zap.Error(err),
				zap.Any("status", s),
			)
			return fmt.Errorf("stream err from client %v: %w", node.Name, err)
		}
	}
}

func watchLayers(
	ctx context.Context,
	eg *errgroup.Group,
	node *cluster.NodeClient,
	logger *zap.Logger,
	collector func(*pb.LayerStreamResponse) (bool, error),
) {
	eg.Go(func() error {
		return layersStream(ctx, node, logger, collector)
	})
}

func layersStream(
	ctx context.Context,
	node *cluster.NodeClient,
	logger *zap.Logger,
	collector func(*pb.LayerStreamResponse) (bool, error),
) error {
	retries := 0
BACKOFF:
	meshapi := pb.NewMeshServiceClient(node.PubConn())
	layers, err := meshapi.LayerStream(ctx, &pb.LayerStreamRequest{})
	if err != nil {
		return err
	}
	for {
		layer, err := layers.Recv()
		s, ok := status.FromError(err)
		if !ok {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("unknown error: %w", err)
		}
		switch s.Code() {
		case codes.OK:
			if cont, err := collector(layer); !cont {
				return err
			}
		case codes.Canceled, codes.DeadlineExceeded:
			return nil
		case codes.Unavailable:
			if retries == attempts {
				return errors.New("layer stream unavailable")
			}
			retries++
			time.Sleep(retryBackoff)
			goto BACKOFF
		default:
			logger.Warn(
				"layer stream error",
				zap.String("client", node.Name),
				zap.Error(err),
				zap.Any("status", s),
			)
			return fmt.Errorf("stream err from client %v: %w", node.Name, err)
		}
	}
}

func malfeasanceStream(
	ctx context.Context,
	node *cluster.NodeClient,
	logger *zap.Logger,
	collector func(*pb.MalfeasanceStreamResponse) (bool, error),
) error {
	retries := 0
BACKOFF:
	meshapi := pb.NewMeshServiceClient(node.PubConn())
	proofs, err := meshapi.MalfeasanceStream(ctx, &pb.MalfeasanceStreamRequest{IncludeProof: true})
	if err != nil {
		return err
	}
	for {
		proof, err := proofs.Recv()
		s, ok := status.FromError(err)
		if !ok {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("unknown error: %w", err)
		}
		switch s.Code() {
		case codes.OK:
			if cont, err := collector(proof); !cont {
				return err
			}
		case codes.Canceled, codes.DeadlineExceeded:
			return nil
		case codes.Unavailable:
			if retries == attempts {
				return errors.New("malfeasance stream unavailable")
			}
			retries++
			time.Sleep(retryBackoff)
			goto BACKOFF
		default:
			logger.Warn(
				"malfeasance stream error",
				zap.String("client", node.Name),
				zap.Error(err),
				zap.Any("status", s),
			)
			return fmt.Errorf("stream err from client %v: %w", node.Name, err)
		}
	}
}

func waitGenesis(ctx *testcontext.Context, node *cluster.NodeClient) error {
	svc := pb.NewMeshServiceClient(node.PubConn())
	resp, err := svc.GenesisTime(ctx, &pb.GenesisTimeRequest{})
	if err != nil {
		return err
	}
	genesis := time.Unix(int64(resp.Unixtime.Value), 0)
	now := time.Now()
	if !genesis.After(now) {
		return nil
	}
	ctx.Log.Debugw("waiting for genesis", "now", now, "genesis", genesis)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(genesis.Sub(now)):
		return nil
	}
}

func waitLayer(ctx *testcontext.Context, node *cluster.NodeClient, lid uint32) error {
	svc := pb.NewMeshServiceClient(node.PubConn())
	resp, err := svc.GenesisTime(ctx, &pb.GenesisTimeRequest{})
	if err != nil {
		return err
	}
	lyrTime := time.Unix(int64(resp.Unixtime.Value), 0).
		Add(time.Duration(lid) * testcontext.LayerDuration.Get(ctx.Parameters))

	now := time.Now()
	if !lyrTime.After(now) {
		return nil
	}
	ctx.Log.Debugw("waiting for layer", "now", now, "layer time", lyrTime, "layer", lid)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(lyrTime.Sub(now)):
		return nil
	}
}

func waitTransaction(ctx context.Context, eg *errgroup.Group, client *cluster.NodeClient, id []byte) {
	eg.Go(func() error {
		api := pb.NewTransactionServiceClient(client.PubConn())
		rsts, err := api.StreamResults(ctx, &pb.TransactionResultsRequest{Watch: true, Id: id})
		if err != nil {
			return err
		}
		_, err = rsts.Recv()
		if err != nil {
			return fmt.Errorf("stream error on receiving result %s: %w", client.Name, err)
		}
		return nil
	})
}

func watchTransactionResults(
	ctx context.Context,
	client *cluster.NodeClient,
	log *zap.Logger,
	collector func(*pb.TransactionResult) (bool, error),
) error {
	retries := 0
BACKOFF:
	api := pb.NewTransactionServiceClient(client.PubConn())
	rsts, err := api.StreamResults(ctx, &pb.TransactionResultsRequest{Watch: true})
	if err != nil {
		return err
	}
	for {
		rst, err := rsts.Recv()
		s, ok := status.FromError(err)
		if !ok {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("unknown error: %w", err)
		}
		switch s.Code() {
		case codes.OK:
			if cont, err := collector(rst); !cont {
				return err
			}
		case codes.Canceled, codes.DeadlineExceeded:
			return nil
		case codes.Unavailable:
			if retries == attempts {
				return errors.New("transaction results unavailable")
			}
			retries++
			time.Sleep(retryBackoff)
			goto BACKOFF
		default:
			log.Warn(
				"transactions stream error",
				zap.String("client", client.Name),
				zap.Error(err),
				zap.Any("status", s),
			)
			return fmt.Errorf("stream error on receiving result %s: %w", client.Name, err)
		}
	}
}

func watchProposals(
	ctx context.Context,
	eg *errgroup.Group,
	client *cluster.NodeClient,
	log *zap.Logger,
	collector func(*pb.Proposal) (bool, error),
) {
	eg.Go(func() error {
		retries := 0
	BACKOFF:
		dbg := pb.NewDebugServiceClient(client.PrivConn())
		proposals, err := dbg.ProposalsStream(ctx, &emptypb.Empty{})
		if err != nil {
			return fmt.Errorf("proposal stream for %s: %w", client.Name, err)
		}
		for {
			proposal, err := proposals.Recv()
			s, ok := status.FromError(err)
			if !ok {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("unknown error: %w", err)
			}
			switch s.Code() {
			case codes.OK:
				if cont, err := collector(proposal); !cont {
					return err
				}
			case codes.Canceled, codes.DeadlineExceeded:
				return nil
			case codes.Unavailable:
				if retries == attempts {
					return errors.New("proposal stream unavailable")
				}
				retries++
				time.Sleep(retryBackoff)
				goto BACKOFF
			default:
				log.Warn(
					"proposals stream error",
					zap.String("client", client.Name),
					zap.Error(err),
					zap.Any("status", s),
				)
				return fmt.Errorf("proposal event for %s: %w", client.Name, err)
			}
		}
	})
}

func prettyHex(buf []byte) string {
	return fmt.Sprintf("0x%x", buf)
}

func scheduleChaos(
	ctx context.Context,
	eg *errgroup.Group,
	client *cluster.NodeClient,
	logger *zap.Logger,
	from, to uint32,
	action func(context.Context) (chaos.Teardown, error),
) {
	var teardown chaos.Teardown
	watchLayers(ctx, eg, client, logger, func(layer *pb.LayerStreamResponse) (bool, error) {
		if layer.Layer.Number.Number == from && teardown == nil {
			var err error
			teardown, err = action(ctx)
			if err != nil {
				return false, err
			}
		}
		if layer.Layer.Number.Number == to {
			if err := teardown(ctx); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	})
}

func currentLayer(ctx context.Context, tb testing.TB, client *cluster.NodeClient) uint32 {
	tb.Helper()
	response, err := pb.NewMeshServiceClient(client.PubConn()).CurrentLayer(ctx, &pb.CurrentLayerRequest{})
	require.NoError(tb, err)
	return response.Layernum.Number
}

func waitAll(tctx *testcontext.Context, cl *cluster.Cluster) error {
	var eg errgroup.Group
	for i := range cl.Total() {
		eg.Go(func() error {
			return cl.Wait(tctx, i)
		})
	}
	return eg.Wait()
}

func nextFirstLayer(current, size uint32) uint32 {
	if over := current % size; over != 0 {
		current += size - over
	}
	return current
}

func getNonce(ctx context.Context, client *cluster.NodeClient, address types.Address) (uint64, error) {
	gstate := pb.NewGlobalStateServiceClient(client.PubConn())
	resp, err := gstate.Account(ctx, &pb.AccountRequest{AccountId: &pb.AccountId{Address: address.String()}})
	if err != nil {
		return 0, err
	}
	return resp.AccountWrapper.StateProjected.Counter, nil
}

func currentBalance(ctx context.Context, client *cluster.NodeClient, address types.Address) (uint64, error) {
	gstate := pb.NewGlobalStateServiceClient(client.PubConn())
	resp, err := gstate.Account(ctx, &pb.AccountRequest{AccountId: &pb.AccountId{Address: address.String()}})
	if err != nil {
		return 0, err
	}
	return resp.AccountWrapper.StateCurrent.Balance.Value, nil
}

func submitSpawn(ctx context.Context, cluster *cluster.Cluster, account int, client *cluster.NodeClient) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := submitTransaction(ctx,
		wallet.SelfSpawn(cluster.Private(account), 0, sdk.WithGenesisID(cluster.GenesisID())),
		client,
	)
	return err
}

func submitSpend(
	ctx context.Context,
	cluster *cluster.Cluster,
	account int,
	receiver types.Address,
	amount, nonce uint64,
	client *cluster.NodeClient,
) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx := wallet.Spend(cluster.Private(account), receiver, amount, nonce, sdk.WithGenesisID(cluster.GenesisID()))
	_, err := submitTransaction(ctx, tx, client)
	return err
}

func syncedNodes(ctx context.Context, cl *cluster.Cluster) []*cluster.NodeClient {
	var synced []*cluster.NodeClient
	for i := 0; i < cl.Total(); i++ {
		if !isSynced(ctx, cl.Client(i)) {
			continue
		}
		synced = append(synced, cl.Client(i))
	}
	return synced
}

func isSynced(ctx context.Context, node *cluster.NodeClient) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	svc := pb.NewNodeServiceClient(node.PubConn())
	resp, err := svc.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return false
	}
	return resp.Status.IsSynced
}

func getLayer(ctx context.Context, node *cluster.NodeClient, lid uint32) (*pb.Layer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	layer := &pb.LayerNumber{Number: lid}
	msvc := pb.NewMeshServiceClient(node.PubConn())
	lresp, err := msvc.LayersQuery(ctx, &pb.LayersQueryRequest{StartLayer: layer, EndLayer: layer})
	if err != nil {
		return nil, err
	}
	if len(lresp.Layer) != 1 {
		return nil, fmt.Errorf("request was made for one layer (%d)", layer.Number)
	}
	return lresp.Layer[0], nil
}

func getVerifiedLayer(ctx context.Context, node *cluster.NodeClient) (*pb.Layer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	svc := pb.NewNodeServiceClient(node.PubConn())
	resp, err := svc.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return getLayer(ctx, node, resp.Status.VerifiedLayer.Number)
}

type txClient struct {
	account cluster.Account
	node    *cluster.NodeClient
}

func (c *txClient) nonce(ctx context.Context) (uint64, error) {
	return getNonce(ctx, c.node, c.account.Address)
}

func (c *txClient) submit(ctx context.Context, tx []byte) (*txRequest, error) {
	var (
		txid []byte
		err  error
	)
	for i := 0; i < attempts; i++ {
		if txid, err = submitTransaction(ctx, tx, c.node); err == nil {
			return &txRequest{
				node: c.node,
				txid: txid,
			}, nil
		}
	}
	return nil, fmt.Errorf("submit to node %s: %w", c.node.Name, err)
}

type txRequest struct {
	node *cluster.NodeClient
	txid []byte

	rst *pb.TransactionResult
}

func (r *txRequest) wait(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := pb.NewTransactionServiceClient(r.node.PubConn())
	stream, err := client.StreamResults(ctx, &pb.TransactionResultsRequest{
		Id:    r.txid,
		Watch: true,
	})
	if err != nil {
		return err
	}
	rst, err := stream.Recv()
	if err != nil {
		return err
	}
	r.rst = rst
	return nil
}

func (r *txRequest) result(ctx context.Context) (*pb.TransactionResult, error) {
	if r.rst != nil {
		return r.rst, nil
	}
	client := pb.NewTransactionServiceClient(r.node.PubConn())
	stream, err := client.StreamResults(ctx, &pb.TransactionResultsRequest{
		Id: r.txid,
	})
	if err != nil {
		return nil, err
	}
	rst, err := stream.Recv()
	if err != nil {
		// eof without result - transaction wasn't applied yet
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	r.rst = rst
	return rst, nil
}
