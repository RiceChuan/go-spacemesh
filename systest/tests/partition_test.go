package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pb "github.com/spacemeshos/api/release/go/spacemesh/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/systest/chaos"
	"github.com/spacemeshos/go-spacemesh/systest/cluster"
	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

func testPartition(tb testing.TB, tctx *testcontext.Context, cl *cluster.Cluster, pct int, wait uint32) {
	require.Greater(tb, cl.Bootnodes(), 1)
	layersPerEpoch := uint32(testcontext.LayersPerEpoch.Get(tctx.Parameters))

	var (
		first      = layersPerEpoch * 2
		startSplit = uint32(4*layersPerEpoch) - 1
		rejoin     = startSplit + layersPerEpoch
		last       = rejoin + (wait-1)*layersPerEpoch
		stop       = rejoin + wait*layersPerEpoch
	)

	tctx.Log.Debug("scheduling chaos...")
	layerDuration := testcontext.LayerDuration.Get(tctx.Parameters)
	deadline := cl.Genesis().Add(time.Duration(stop+2*layersPerEpoch) * layerDuration) // add 2 epochs of buffer
	ctx, cancel := context.WithDeadline(tctx, deadline)
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)
	// make sure the first boot node is in the 2nd partition so the poet proof can be broadcast to both splits
	split := pct*cl.Total()/100 + 1
	scheduleChaos(ctx, eg, cl.Client(0), tctx.Log.Desugar(), startSplit, rejoin,
		func(ctx context.Context) (chaos.Teardown, error) {
			var (
				left  []string
				right = []string{cl.Client(0).Name}
			)
			for i := 1; i < cl.Total(); i++ {
				if i < split {
					left = append(left, cl.Client(i).Name)
				} else {
					right = append(right, cl.Client(i).Name)
				}
			}
			tctx.Log.Debugw("long partition",
				"percentage", pct,
				"split", startSplit,
				"rejoin", rejoin,
				"last", last,
				"stop", stop,
				"left", left,
				"right", right,
			)
			return chaos.Partition2(tctx, fmt.Sprintf("split-%v-%v", pct, 100-pct), left, right)
		},
	)

	// start sending transactions
	tctx.Log.Debug("sending transactions...")
	receiver := types.GenerateAddress([]byte{11, 1, 1})

	ctx2, cancel := context.WithDeadline(tctx, deadline)
	eg2, ctx2 := errgroup.WithContext(ctx2)
	defer cancel()
	eg2.Go(func() error {
		return sendTransactions(ctx2, tctx.Log.Desugar(), cl, first, stop, receiver, 10, 100)
	})

	type stateUpdate struct {
		layer  uint32
		hash   types.Hash32
		client string
	}
	numLayers := stop - types.GetEffectiveGenesis().Uint32()
	// assuming each client can update state for the same layer up to 10 times
	stateCh := make(chan *stateUpdate, uint32(cl.Total())*numLayers*10)
	tctx.Log.Debug("listening to state hashes...")
	for i := range cl.Total() {
		node := cl.Client(i)

		eg.Go(func() error {
			return stateHashStream(ctx, node, tctx.Log.Desugar(),
				func(state *pb.GlobalStateStreamResponse) (bool, error) {
					data := state.Datum.Datum
					require.IsType(tb, &pb.GlobalStateData_GlobalState{}, data)

					resp := data.(*pb.GlobalStateData_GlobalState)
					layer := resp.GlobalState.Layer.Number
					if layer > stop {
						return false, nil
					}

					stateHash := types.BytesToHash(resp.GlobalState.RootHash)
					tctx.Log.Debugw("state hash collected",
						"client", node.Name,
						"layer", layer,
						"state", stateHash.ShortString(),
					)
					select {
					case stateCh <- &stateUpdate{
						layer:  layer,
						hash:   stateHash,
						client: node.Name,
					}: // continue
					case <-ctx.Done():
						return false, ctx.Err()
					default:
						tctx.Log.Errorw("state hash channel is full",
							"client", node.Name,
							"layer", layer,
							"state", stateHash.ShortString(),
						)
						return false, errors.New("state hash channel is full")
					}
					return true, nil
				},
			)
		})
	}

	finalErr := eg.Wait()
	close(stateCh)
	// this map contains info for debugging only
	hashes := make(map[uint32]map[types.Hash32][]string)
	// this map contains the latest hash reported by each client/layer
	latestStates := make(map[string]map[uint32]types.Hash32)
	for update := range stateCh {
		if _, ok := hashes[update.layer]; !ok {
			hashes[update.layer] = make(map[types.Hash32][]string)
		}
		if _, ok := hashes[update.layer][update.hash]; !ok {
			hashes[update.layer][update.hash] = []string{}
		}
		hashes[update.layer][update.hash] = append(hashes[update.layer][update.hash], update.client)

		if _, ok := latestStates[update.client]; !ok {
			latestStates[update.client] = make(map[uint32]types.Hash32)
		}
		latestStates[update.client][update.layer] = update.hash
	}
	for layer := layersPerEpoch * 2; layer <= last; layer++ {
		tctx.Log.Debugw("client states",
			"layer", layer,
			"num_states", len(hashes[layer]),
			"states", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
				for hash, clients := range hashes[layer] {
					encoder.AddString("hash", hash.ShortString())
					encoder.AddInt("num_clients", len(clients))
					encoder.AddArray("clients", zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
						for _, c := range clients {
							encoder.AppendString(c)
						}
						return nil
					}))
				}
				return nil
			}))
	}
	refState := latestStates[cl.Client(0).Name]
	pass := true
	for i := 1; i < cl.Total(); i++ {
		clientState := latestStates[cl.Client(i).Name]
		agree := true
		for layer := layersPerEpoch * 2; layer <= last; layer++ {
			if clientState[layer] != refState[layer] {
				tctx.Log.Errorw("client state differs from ref state",
					"client", cl.Client(i).Name,
					"ref_client", cl.Client(0).Name,
					"layer", layer,
					"client_hash", clientState[layer],
					"ref_hash", refState[layer],
				)
				agree = false
				break
			}
		}
		if agree {
			tctx.Log.Debugw("client agreed with ref client on all layers",
				"client", cl.Client(i).Name,
				"ref_client", cl.Client(0).Name,
			)
		}
		pass = pass && agree
	}
	require.NoError(tb, finalErr)
	require.True(tb, pass)
	require.NoError(tb, eg2.Wait())
}

// TestPartition_30_70 tests the network partitioning with 30% and 70% of the nodes in each partition.
func TestPartition_30_70(t *testing.T) {
	t.Parallel()

	tctx := testcontext.New(t)
	if tctx.ClusterSize > 20 {
		tctx.Log.Info("cluster size changed to 20")
		tctx.ClusterSize = 20
	}
	cl, err := cluster.ReuseWait(tctx, cluster.WithKeys(tctx.ClusterSize))
	require.NoError(t, err)
	// TODO: re-assess the number of epoch required for healing.
	testPartition(t, tctx, cl, 30, 6)
}

// TestPartition_50_50 tests the network partitioning with 50% of the nodes in each partition.
func TestPartition_50_50(t *testing.T) {
	t.Parallel()

	tctx := testcontext.New(t)
	if tctx.ClusterSize > 20 {
		tctx.Log.Info("cluster size changed to 20")
		tctx.ClusterSize = 20
	}
	cl, err := cluster.ReuseWait(tctx, cluster.WithKeys(tctx.ClusterSize))
	require.NoError(t, err)
	// TODO: re-assess the number of epoch required for healing.
	testPartition(t, tctx, cl, 50, 6)
}
