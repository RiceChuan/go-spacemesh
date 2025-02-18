package v2alpha1

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	spacemeshv2alpha1 "github.com/spacemeshos/api/release/go/spacemesh/v2alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/spacemeshos/go-spacemesh/common/fixture"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/events"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/statesql"
)

func TestActivationService_List(t *testing.T) {
	setup := func(t *testing.T) (spacemeshv2alpha1.ActivationServiceClient, []types.ActivationTx) {
		db := statesql.InMemoryTest(t)

		gen := fixture.NewAtxsGenerator()
		activations := make([]types.ActivationTx, 100)
		for i := range activations {
			atx := gen.Next()
			require.NoError(t, atxs.Add(db, atx, types.AtxBlob{}))
			activations[i] = *atx
		}

		goldenAtx := types.ATXID{2, 3, 4}
		svc := NewActivationService(db, goldenAtx)
		cfg, cleanup := launchServer(t, svc)
		t.Cleanup(cleanup)

		conn := dialGrpc(t, cfg)
		return spacemeshv2alpha1.NewActivationServiceClient(conn), activations
	}

	t.Run("limit set too high", func(t *testing.T) {
		client, _ := setup(t)
		_, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{Limit: 200})
		require.Error(t, err)

		s, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, s.Code())
		require.Equal(t, "limit is capped at 100", s.Message())
	})

	t.Run("no limit set", func(t *testing.T) {
		client, _ := setup(t)
		_, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{})
		require.Error(t, err)

		s, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, s.Code())
		require.Equal(t, "limit must be set to <= 100", s.Message())
	})

	t.Run("limit and offset", func(t *testing.T) {
		client, _ := setup(t)
		list, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{
			Limit:  25,
			Offset: 50,
		})
		require.NoError(t, err)
		require.Len(t, list.Activations, 25)
	})

	t.Run("all", func(t *testing.T) {
		client, activations := setup(t)
		list, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{Limit: 100})
		require.NoError(t, err)
		require.Equal(t, len(activations), len(list.Activations))
	})

	t.Run("coinbase", func(t *testing.T) {
		client, activations := setup(t)
		list, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{
			Limit:    1,
			Coinbase: activations[3].Coinbase.String(),
		})
		require.NoError(t, err)
		require.Equal(t, activations[3].ID().Bytes(), list.GetActivations()[0].GetId())
	})

	t.Run("smesherId", func(t *testing.T) {
		client, activations := setup(t)
		list, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{
			Limit:     1,
			SmesherId: [][]byte{activations[1].SmesherID.Bytes()},
		})
		require.NoError(t, err)
		require.Equal(t, activations[1].ID().Bytes(), list.GetActivations()[0].GetId())
	})

	t.Run("id", func(t *testing.T) {
		client, activations := setup(t)
		list, err := client.List(context.Background(), &spacemeshv2alpha1.ActivationRequest{
			Limit: 1,
			Id:    [][]byte{activations[3].ID().Bytes()},
		})
		require.NoError(t, err)
		require.Equal(t, activations[3].ID().Bytes(), list.GetActivations()[0].GetId())
	})
}

func TestActivationStreamService_Stream(t *testing.T) {
	setup := func(t *testing.T, db sql.Executor) spacemeshv2alpha1.ActivationStreamServiceClient {
		gen := fixture.NewAtxsGenerator()
		activations := make([]types.ActivationTx, 100)
		for i := range activations {
			atx := gen.Next()
			require.NoError(t, atxs.Add(db, atx, types.AtxBlob{}))
			activations[i] = *atx
		}

		svc := NewActivationStreamService(db)
		cfg, cleanup := launchServer(t, svc)
		t.Cleanup(cleanup)

		conn := dialGrpc(t, cfg)
		return spacemeshv2alpha1.NewActivationStreamServiceClient(conn)
	}

	t.Run("all", func(t *testing.T) {
		events.InitializeReporter()
		t.Cleanup(events.CloseEventReporter)

		client := setup(t, statesql.InMemoryTest(t))

		stream, err := client.Stream(context.Background(), &spacemeshv2alpha1.ActivationStreamRequest{})
		require.NoError(t, err)

		var i int
		for {
			_, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			i++
		}
		require.Equal(t, 100, i)
	})

	t.Run("watch", func(t *testing.T) {
		events.InitializeReporter()
		t.Cleanup(events.CloseEventReporter)

		db := statesql.InMemoryTest(t)
		client := setup(t, db)

		const (
			start = 100
			n     = 10
		)

		gen := fixture.NewAtxsGenerator().WithEpochs(start, 10)
		var streamed []*events.ActivationTx
		for i := 0; i < n; i++ {
			atx := gen.Next()
			require.NoError(t, atxs.Add(db, atx, types.AtxBlob{}))
			streamed = append(streamed, &events.ActivationTx{ActivationTx: atx})
		}

		for _, tc := range []struct {
			desc    string
			request *spacemeshv2alpha1.ActivationStreamRequest
		}{
			{
				desc: "ID",
				request: &spacemeshv2alpha1.ActivationStreamRequest{
					Id:         [][]byte{streamed[3].ID().Bytes()},
					StartEpoch: start,
					Watch:      true,
				},
			},
			{
				desc: "SmesherId",
				request: &spacemeshv2alpha1.ActivationStreamRequest{
					SmesherId:  [][]byte{streamed[3].SmesherID.Bytes()},
					StartEpoch: start,
					Watch:      true,
				},
			},
			{
				desc: "Coinbase",
				request: &spacemeshv2alpha1.ActivationStreamRequest{
					Coinbase:   streamed[3].Coinbase.String(),
					StartEpoch: start,
					Watch:      true,
				},
			},
		} {
			t.Run(tc.desc, func(t *testing.T) {
				stream, err := client.Stream(context.Background(), tc.request)
				require.NoError(t, err)
				_, err = stream.Header()
				require.NoError(t, err)

				var expect []*types.ActivationTx
				for _, rst := range streamed {
					require.NoError(t, events.ReportNewActivation(rst.ActivationTx))
					matcher := atxsMatcher{tc.request, context.Background()}
					if matcher.match(rst) {
						expect = append(expect, rst.ActivationTx)
					}
				}

				for _, rst := range expect {
					received, err := stream.Recv()
					require.NoError(t, err)
					require.Equal(t, toAtx(rst).String(), received.String())
				}
			})
		}
	})
}

func TestActivationService_ActivationsCount(t *testing.T) {
	db := statesql.InMemoryTest(t)
	ctx := context.Background()

	genEpoch3 := fixture.NewAtxsGenerator().WithEpochs(3, 1)
	epoch3ATXs := make([]types.ActivationTx, 30)
	for i := range epoch3ATXs {
		atx := genEpoch3.Next()
		require.NoError(t, atxs.Add(db, atx, types.AtxBlob{}))
		epoch3ATXs[i] = *atx
	}

	genEpoch5 := fixture.NewAtxsGenerator().WithSeed(time.Now().UnixNano()+1).
		WithEpochs(5, 1)
	epoch5ATXs := make([]types.ActivationTx, 10) // ensure the number here is different from above
	for i := range epoch5ATXs {
		atx := genEpoch5.Next()
		require.NoError(t, atxs.Add(db, atx, types.AtxBlob{}))
		epoch5ATXs[i] = *atx
	}

	goldenAtx := types.ATXID{2, 3, 4}
	svc := NewActivationService(db, goldenAtx)
	cfg, cleanup := launchServer(t, svc)
	t.Cleanup(cleanup)

	conn := dialGrpc(t, cfg)
	client := spacemeshv2alpha1.NewActivationServiceClient(conn)

	t.Run("count without filter", func(t *testing.T) {
		count, err := client.ActivationsCount(ctx, &spacemeshv2alpha1.ActivationsCountRequest{})
		require.NoError(t, err)
		require.Equal(t, len(epoch3ATXs)+len(epoch5ATXs), int(count.Count))
	})

	t.Run("count with filter", func(t *testing.T) {
		epoch := uint32(3)
		epoch3Count, err := client.ActivationsCount(ctx, &spacemeshv2alpha1.ActivationsCountRequest{
			Epoch: &epoch,
		})
		require.NoError(t, err)
		require.Len(t, epoch3ATXs, int(epoch3Count.Count))

		epoch = uint32(5)
		epoch5Count, err := client.ActivationsCount(ctx, &spacemeshv2alpha1.ActivationsCountRequest{
			Epoch: &epoch,
		})
		require.NoError(t, err)
		require.Len(t, epoch5ATXs, int(epoch5Count.Count))

		require.NotEqual(t, int(epoch3Count.Count), int(epoch5Count.Count))
	})
}

func TestActivationService_Highest(t *testing.T) {
	t.Run("max tick height", func(t *testing.T) {
		db := statesql.InMemoryTest(t)
		ctx := context.Background()

		goldenAtx := types.ATXID{2, 3, 4}
		svc := NewActivationService(db, goldenAtx)
		cfg, cleanup := launchServer(t, svc)
		t.Cleanup(cleanup)

		conn := dialGrpc(t, cfg)
		client := spacemeshv2alpha1.NewActivationServiceClient(conn)

		atx := &types.ActivationTx{
			Sequence:     rand.Uint64(),
			PublishEpoch: 0,
			Coinbase:     types.GenerateAddress(types.RandomBytes(32)),
			NumUnits:     rand.Uint32(),
		}
		id := types.RandomATXID()
		atx.SetID(id)
		require.NoError(t, atxs.Add(db, atx, types.AtxBlob{}))

		res, err := client.Highest(ctx, &spacemeshv2alpha1.HighestRequest{})
		require.NoError(t, err)
		require.Equal(t, atx.ID().Bytes(), res.Activation.Id)
		require.Equal(t, atx.PublishEpoch.Uint32(), res.Activation.PublishEpoch)
		require.Equal(t, atx.SmesherID.Bytes(), res.Activation.SmesherId)
		require.Equal(t, atx.Coinbase.String(), res.Activation.Coinbase)
		require.Equal(t, atx.NumUnits, res.Activation.NumUnits)
		require.Equal(t, atx.Weight, res.Activation.Weight)
		require.Equal(t, atx.TickHeight(), res.Activation.Height)
	})
	t.Run("returns golden atx on error", func(t *testing.T) {
		db := statesql.InMemoryTest(t)
		ctx := context.Background()

		goldenAtx := types.ATXID{2, 3, 4}
		svc := NewActivationService(db, goldenAtx)
		cfg, cleanup := launchServer(t, svc)
		t.Cleanup(cleanup)

		conn := dialGrpc(t, cfg)
		client := spacemeshv2alpha1.NewActivationServiceClient(conn)

		res, err := client.Highest(ctx, &spacemeshv2alpha1.HighestRequest{})
		require.NoError(t, err)
		require.Equal(t, goldenAtx.Bytes(), res.Activation.Id)
		require.Empty(t, res.Activation.PublishEpoch)
		require.Empty(t, res.Activation.SmesherId)
		require.Empty(t, res.Activation.Coinbase)
		require.Empty(t, res.Activation.NumUnits)
		require.Empty(t, res.Activation.Weight)
		require.Empty(t, res.Activation.Height)
	})
}
