package activation

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spacemeshos/post/shared"
	"github.com/spacemeshos/post/verifying"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/spacemeshos/go-spacemesh/activation/wire"
	"github.com/spacemeshos/go-spacemesh/atxsdata"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/fetch"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/malfeasance"
	"github.com/spacemeshos/go-spacemesh/sql/marriage"
	"github.com/spacemeshos/go-spacemesh/sql/statesql"
)

type v2TestHandler struct {
	*HandlerV2

	tb testing.TB
	handlerMocks
}

type marriedId struct {
	signer *signing.EdSigner
	refAtx *wire.ActivationTxV2
}

const (
	tickSize   = 20
	poetLeaves = 200
)

func newV2TestHandler(tb testing.TB, golden types.ATXID) *v2TestHandler {
	lg := zaptest.NewLogger(tb)
	cdb := datastore.NewCachedDB(statesql.InMemoryTest(tb), lg)
	tb.Cleanup(func() { assert.NoError(tb, cdb.Close()) })
	mocks := newTestHandlerMocks(tb, golden)
	return &v2TestHandler{
		HandlerV2: &HandlerV2{
			local:           "localID",
			cdb:             cdb,
			atxsdata:        atxsdata.New(),
			edVerifier:      signing.NewEdVerifier(),
			clock:           mocks.mClock,
			tickSize:        tickSize,
			goldenATXID:     golden,
			nipostValidator: mocks.mValidator,
			logger:          lg,
			fetcher:         mocks.mockFetch,
			beacon:          mocks.mBeacon,
			tortoise:        mocks.mTortoise,
			malPublisher:    mocks.mMalPublish,
		},
		tb:           tb,
		handlerMocks: mocks,
	}
}

func (h *handlerMocks) expectFetchDeps(atx *wire.ActivationTxV2) {
	h.mockFetch.EXPECT().RegisterPeerHashes(gomock.Any(), gomock.Any())
	h.mockFetch.EXPECT().GetPoetProof(gomock.Any(), atx.NIPosts[0].Challenge)
	_, atxDeps := (&HandlerV2{goldenATXID: h.goldenATXID}).collectAtxDeps(atx)
	if len(atxDeps) != 0 {
		h.mockFetch.EXPECT().GetAtxs(gomock.Any(), gomock.InAnyOrder(atxDeps), gomock.Any())
	}
}

func (h *handlerMocks) expectVerifyNIPoST(atx *wire.ActivationTxV2) {
	h.mValidator.EXPECT().PostV2(
		gomock.Any(),
		atx.SmesherID,
		gomock.Any(),
		wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
		atx.NIPosts[0].Challenge.Bytes(),
		atx.NIPosts[0].Posts[0].NumUnits,
		gomock.Any(),
	)
	h.mValidator.EXPECT().PoetMembership(
		gomock.Any(),
		gomock.Any(),
		atx.NIPosts[0].Challenge,
		gomock.Any(),
	).Return(poetLeaves, nil)
}

func (h *handlerMocks) expectVerifyNIPoSTs(
	atx *wire.ActivationTxV2,
	equivocationSet []types.NodeID,
	poetLeaves []uint64,
) {
	for i, nipost := range atx.NIPosts {
		for _, post := range nipost.Posts {
			h.mValidator.EXPECT().PostV2(
				gomock.Any(),
				equivocationSet[post.MarriageIndex],
				gomock.Any(),
				wire.PostFromWireV1(&post.Post),
				nipost.Challenge.Bytes(),
				post.NumUnits,
				gomock.Any(),
			)
		}
		h.mValidator.EXPECT().PoetMembership(
			gomock.Any(),
			gomock.Any(),
			nipost.Challenge,
			gomock.Any(),
		).Return(poetLeaves[i], nil)
	}
}

func (h *handlerMocks) expectStoreAtxV2(atx *wire.ActivationTxV2) {
	h.mBeacon.EXPECT().OnAtx(gomock.Cond(func(a *types.ActivationTx) bool {
		return a.ID() == atx.ID()
	}))
	h.mTortoise.EXPECT().OnAtx(atx.PublishEpoch+1, atx.ID(), gomock.Any())
	h.mValidator.EXPECT().IsVerifyingFullPost().Return(false)
}

func (h *handlerMocks) expectInitialAtxV2(atx *wire.ActivationTxV2) {
	h.mClock.EXPECT().CurrentLayer().Return(atx.PublishEpoch.FirstLayer())
	h.mValidator.EXPECT().VRFNonceV2(
		atx.SmesherID,
		atx.Initial.CommitmentATX,
		atx.VRFNonce,
		atx.NIPosts[0].Posts[0].NumUnits,
	)
	h.mValidator.EXPECT().PostV2(
		gomock.Any(),
		atx.SmesherID,
		atx.Initial.CommitmentATX,
		wire.PostFromWireV1(&atx.Initial.Post),
		shared.ZeroChallenge,
		atx.NIPosts[0].Posts[0].NumUnits,
		gomock.Any(),
	)

	h.expectFetchDeps(atx)
	h.expectVerifyNIPoST(atx)
	h.expectStoreAtxV2(atx)
}

func (h *handlerMocks) expectAtxV2(atx *wire.ActivationTxV2) {
	h.mClock.EXPECT().CurrentLayer().Return(atx.PublishEpoch.FirstLayer())
	h.mValidator.EXPECT().VRFNonceV2(
		atx.SmesherID,
		gomock.Any(),
		atx.VRFNonce,
		atx.NIPosts[0].Posts[0].NumUnits,
	)
	h.expectFetchDeps(atx)
	h.expectVerifyNIPoST(atx)
	h.expectStoreAtxV2(atx)
}

func (h *handlerMocks) expectMergedAtxV2(
	atx *wire.ActivationTxV2,
	equivocationSet []types.NodeID,
	poetLeaves []uint64,
) {
	h.mClock.EXPECT().CurrentLayer().Return(atx.PublishEpoch.FirstLayer())
	h.expectFetchDeps(atx)
	h.mValidator.EXPECT().VRFNonceV2(
		atx.SmesherID,
		gomock.Any(),
		atx.VRFNonce,
		atx.TotalNumUnits(),
	)
	h.expectVerifyNIPoSTs(atx, equivocationSet, poetLeaves)
	h.expectStoreAtxV2(atx)
}

func (h *v2TestHandler) createAndProcessInitial(sig *signing.EdSigner) *wire.ActivationTxV2 {
	h.tb.Helper()
	atx := newInitialATXv2(h.tb, h.handlerMocks.goldenATXID)
	atx.Sign(sig)
	err := h.processInitial(atx)
	require.NoError(h.tb, err)
	return atx
}

func (h *v2TestHandler) processInitial(atx *wire.ActivationTxV2) error {
	h.tb.Helper()
	h.expectInitialAtxV2(atx)
	return h.processATX(context.Background(), peer.ID("peer"), atx, time.Now())
}

func (h *v2TestHandler) processSoloAtx(atx *wire.ActivationTxV2) error {
	h.tb.Helper()
	h.expectAtxV2(atx)
	return h.processATX(context.Background(), peer.ID("peer"), atx, time.Now())
}

func TestHandlerV2_SyntacticallyValidate(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)
	t.Run("rejects invalid signature", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)

		atxHandler := newV2TestHandler(t, golden)
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "signature")
	})
	t.Run("rejects from future", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.PublishEpoch = 100
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer().Return(0)
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "atx publish epoch is too far in the future")
	})
	t.Run("rejects empty positioning ATX", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.PositioningATX = types.EmptyATXID
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "empty positioning atx")
	})
	t.Run("reject golden previous ATX", func(t *testing.T) {
		t.Parallel()
		atx := newSoloATXv2(t, 0, golden, golden)
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "previous atx[0] is the golden ATX")
	})
	t.Run("reject empty previous ATX", func(t *testing.T) {
		t.Parallel()
		atx := newSoloATXv2(t, 0, types.EmptyATXID, golden)
		atx.PreviousATXs = append(atx.PreviousATXs, types.EmptyATXID)
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "previous atx[0] is empty")
	})
}

func TestHandlerV2_SyntacticallyValidate_InitialAtx(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		atxHandler.mValidator.EXPECT().VRFNonceV2(
			sig.NodeID(),
			atx.Initial.CommitmentATX,
			atx.VRFNonce,
			atx.NIPosts[0].Posts[0].NumUnits,
		)
		atxHandler.mValidator.EXPECT().PostV2(
			context.Background(),
			sig.NodeID(),
			atx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.Initial.Post),
			shared.ZeroChallenge,
			atx.NIPosts[0].Posts[0].NumUnits,
		)
		require.NoError(t, atxHandler.syntacticallyValidate(context.Background(), atx))
	})
	t.Run("rejects previous ATXs", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.PreviousATXs = []types.ATXID{types.RandomATXID()}
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "initial atx must not have previous atxs")

		atx.PreviousATXs = []types.ATXID{types.EmptyATXID}
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err = atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "initial atx must not have previous atxs")
	})
	t.Run("rejects when marriage ATX ref is set", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.MarriageATX = &golden
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "initial atx cannot reference a marriage atx")
	})
	t.Run("rejects when commitment ATX is missing", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.Initial.CommitmentATX = types.EmptyATXID
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "initial atx missing commitment atx")
	})
	t.Run("invalid VRF nonce", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		atxHandler.mValidator.EXPECT().
			VRFNonceV2(
				sig.NodeID(),
				atx.Initial.CommitmentATX,
				atx.VRFNonce,
				atx.NIPosts[0].Posts[0].NumUnits,
			).
			Return(errors.New("invalid nonce"))

		require.ErrorContains(t, atxHandler.syntacticallyValidate(context.Background(), atx), "invalid nonce")
	})
	t.Run("invalid initial PoST", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.mClock.EXPECT().CurrentLayer()
		atxHandler.mValidator.EXPECT().VRFNonceV2(
			sig.NodeID(),
			atx.Initial.CommitmentATX,
			atx.VRFNonce,
			atx.NIPosts[0].Posts[0].NumUnits,
		)
		atxHandler.mValidator.EXPECT().
			PostV2(
				context.Background(),
				sig.NodeID(),
				atx.Initial.CommitmentATX,
				wire.PostFromWireV1(&atx.Initial.Post),
				shared.ZeroChallenge,
				atx.NIPosts[0].Posts[0].NumUnits,
			).
			Return(errors.New("invalid post"))
		require.ErrorContains(t, atxHandler.syntacticallyValidate(context.Background(), atx), "invalid post")
	})
}

func TestHandlerV2_SyntacticallyValidate_SoloAtx(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	atxHandler := newV2TestHandler(t, golden)
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	t.Run("valid", func(t *testing.T) {
		atx := newSoloATXv2(t, 0, types.RandomATXID(), types.RandomATXID())
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.NoError(t, err)
	})
	t.Run("must have 1 previous ATX", func(t *testing.T) {
		atx := newSoloATXv2(t, 0, types.RandomATXID(), types.RandomATXID())
		atx.PreviousATXs = append(atx.PreviousATXs, types.RandomATXID())
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "solo atx must have one previous atx")
	})
	t.Run("rejects when len(NIPoSTs) != 1", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.NIPosts = append(atx.NIPosts, wire.NIPostV2{})
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "solo atx must have one nipost")
	})
	t.Run("rejects when contains more than 1 ID", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.NIPosts[0].Posts = append(atx.NIPosts[0].Posts, wire.SubPostV2{})
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "solo atx must have one post")
	})
	t.Run("rejects when PrevATXIndex != 0", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.NIPosts[0].Posts[0].PrevATXIndex = 1
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err := atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "solo atx post must have prevATXIndex 0")
	})
}

func TestHandlerV2_SyntacticallyValidate_MergedAtx(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	atxHandler := newV2TestHandler(t, golden)
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	t.Run("cannot have marriage", func(t *testing.T) {
		t.Parallel()

		atx := newSoloATXv2(t, 0, types.RandomATXID(), types.RandomATXID())
		atx.MarriageATX = &golden
		atx.Marriages = []wire.MarriageCertificate{{
			Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
		}}
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		err = atxHandler.syntacticallyValidate(context.Background(), atx)
		require.ErrorContains(t, err, "merged atx cannot have marriages")
	})
}

func TestHandlerV2_ProcessSoloATX(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	peer := peer.ID("other")
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	t.Run("initial ATX", func(t *testing.T) {
		t.Parallel()
		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler := newV2TestHandler(t, golden)
		atxHandler.tickSize = tickSize
		atxHandler.expectInitialAtxV2(atx)

		err := atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.NoError(t, err)

		atxFromDb, err := atxs.Get(atxHandler.cdb, atx.ID())
		require.NoError(t, err)
		require.NotNil(t, atx)
		require.Equal(t, atx.ID(), atxFromDb.ID())
		require.Equal(t, atx.Coinbase, atxFromDb.Coinbase)
		require.EqualValues(t, poetLeaves/tickSize, atxFromDb.TickCount)
		require.EqualValues(t, 0+atxFromDb.TickCount, atxFromDb.TickHeight()) // positioning is golden
		require.Equal(t, atx.NIPosts[0].Posts[0].NumUnits, atxFromDb.NumUnits)
		require.EqualValues(t, atx.NIPosts[0].Posts[0].NumUnits*poetLeaves/tickSize, atxFromDb.Weight)

		// processing ATX for the second time should skip checks
		err = atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.NoError(t, err)
	})
	t.Run("second ATX", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		prev := atxHandler.createAndProcessInitial(sig)

		atx := newSoloATXv2(t, prev.PublishEpoch+1, prev.ID(), prev.ID())
		atx.Sign(sig)

		atxHandler.expectAtxV2(atx)
		err := atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.NoError(t, err)

		prevAtx, err := atxs.Get(atxHandler.cdb, prev.ID())
		require.NoError(t, err)
		atxFromDb, err := atxs.Get(atxHandler.cdb, atx.ID())
		require.NoError(t, err)
		require.EqualValues(t, poetLeaves/tickSize, atxFromDb.TickCount)
		require.EqualValues(t, prevAtx.TickHeight(), atxFromDb.BaseTickHeight)
		require.EqualValues(t, prevAtx.TickHeight()+atxFromDb.TickCount, atxFromDb.TickHeight())
		require.Equal(t, atx.NIPosts[0].Posts[0].NumUnits, atxFromDb.NumUnits)
		require.EqualValues(t, atx.NIPosts[0].Posts[0].NumUnits*poetLeaves/tickSize, atxFromDb.Weight)
	})
	t.Run("second ATX, previous checkpointed", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		prev := atxs.CheckpointAtx{
			ID:            types.RandomATXID(),
			CommitmentATX: types.RandomATXID(),
			SmesherID:     sig.NodeID(),
			NumUnits:      100,
			Units:         map[types.NodeID]uint32{sig.NodeID(): 100},
		}
		require.NoError(t, atxs.AddCheckpointed(atxHandler.cdb, &prev))

		atx := newSoloATXv2(t, prev.Epoch+1, prev.ID, golden)
		atx.Sign(sig)
		atxHandler.expectAtxV2(atx)
		err := atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.NoError(t, err)

		atxFromDb, err := atxs.Get(atxHandler.cdb, atx.ID())
		require.NoError(t, err)
		require.Equal(t, atx.TotalNumUnits(), atxFromDb.NumUnits)
	})
	t.Run("second ATX, increases space (nonce valid)", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		prev := atxHandler.createAndProcessInitial(sig)

		atx := newSoloATXv2(t, prev.PublishEpoch+1, prev.ID(), golden)
		atx.NIPosts[0].Posts[0].NumUnits = prev.TotalNumUnits() * 10
		atx.VRFNonce = 7779989
		atx.Sign(sig)
		atxHandler.expectAtxV2(atx)

		err := atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.NoError(t, err)

		atxFromDb, err := atxs.Get(atxHandler.cdb, atx.ID())
		require.NoError(t, err)
		require.EqualValues(t, atx.VRFNonce, atxFromDb.VRFNonce)
		require.Equal(t, min(prev.TotalNumUnits(), atx.TotalNumUnits()), atxFromDb.NumUnits)
	})
	t.Run("second ATX, increases space (nonce invalid)", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		prev := atxHandler.createAndProcessInitial(sig)

		atx := newSoloATXv2(t, prev.PublishEpoch+1, prev.ID(), golden)
		atx.NIPosts[0].Posts[0].NumUnits = prev.TotalNumUnits() * 10
		atx.VRFNonce = 7779989
		atx.Sign(sig)
		atxHandler.mClock.EXPECT().CurrentLayer().Return(postGenesisEpoch.FirstLayer())
		atxHandler.expectFetchDeps(atx)
		atxHandler.expectVerifyNIPoST(atx)
		atxHandler.mValidator.EXPECT().VRFNonceV2(
			sig.NodeID(),
			prev.Initial.CommitmentATX,
			atx.VRFNonce,
			atx.TotalNumUnits(),
		).Return(errors.New("vrf nonce is not valid"))

		err = atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.ErrorContains(t, err, "vrf nonce is not valid")
		require.ErrorIs(t, err, pubsub.ErrValidationReject)

		_, err = atxs.Get(atxHandler.cdb, atx.ID())
		require.ErrorIs(t, err, sql.ErrNotFound)
	})
	t.Run("second ATX, decreases space", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		prev := atxHandler.createAndProcessInitial(sig)

		atx := newSoloATXv2(t, prev.PublishEpoch+1, prev.ID(), golden)
		atx.VRFNonce = uint64(123)
		atx.NIPosts[0].Posts[0].NumUnits = prev.TotalNumUnits() - 1
		atx.Sign(sig)
		atxHandler.expectAtxV2(atx)

		err := atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.NoError(t, err)

		// verify that the ATX was added to the DB and it has the lower effective num units
		atxFromDb, err := atxs.Get(atxHandler.cdb, atx.ID())
		require.NoError(t, err)
		require.Equal(t, min(prev.TotalNumUnits(), atx.TotalNumUnits()), atxFromDb.NumUnits)
		require.EqualValues(t, atx.VRFNonce, atxFromDb.VRFNonce)
	})
	t.Run("can't find positioning ATX", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		atx := newSoloATXv2(t, 0, types.RandomATXID(), types.RandomATXID())
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer()
		atxHandler.expectFetchDeps(atx)
		err := atxHandler.processATX(context.Background(), peer, atx, time.Now())
		require.ErrorContains(t, err, "validating positioning atx")
		require.ErrorIs(t, err, pubsub.ErrValidationReject)

		_, err = atxs.Get(atxHandler.cdb, atx.ID())
		require.ErrorIs(t, err, sql.ErrNotFound)
	})
}

func marryIDs(
	tb testing.TB,
	atxHandler *v2TestHandler,
	signers []*signing.EdSigner,
	golden types.ATXID,
) (marriage *wire.ActivationTxV2, other []*wire.ActivationTxV2) {
	sig := signers[0]
	mATX := newInitialATXv2(tb, golden)
	mATX.Marriages = []wire.MarriageCertificate{{
		Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
	}}

	for _, signer := range signers[1:] {
		atx := atxHandler.createAndProcessInitial(signer)
		other = append(other, atx)
		mATX.Marriages = append(mATX.Marriages, wire.MarriageCertificate{
			ReferenceAtx: atx.ID(),
			Signature:    signer.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
		})
	}

	mATX.Sign(sig)
	atxHandler.expectInitialAtxV2(mATX)
	err := atxHandler.processATX(context.Background(), "", mATX, time.Now())
	require.NoError(tb, err)

	return mATX, other
}

func TestHandlerV2_ProcessMergedATX(t *testing.T) {
	t.Parallel()
	var (
		golden          = types.RandomATXID()
		signers         []*signing.EdSigner
		equivocationSet []types.NodeID
	)
	for range 5 {
		sig, err := signing.NewEdSigner()
		require.NoError(t, err)
		signers = append(signers, sig)
		equivocationSet = append(equivocationSet, sig.NodeID())
	}
	sig := signers[0]

	t.Run("happy case", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		// Marry IDs
		mATX, otherATXs := marryIDs(t, atxHandler, signers, golden)
		previousATXs := []types.ATXID{mATX.ID()}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		totalNumUnits := merged.NIPosts[0].Posts[0].NumUnits
		for i, atx := range otherATXs {
			post := wire.SubPostV2{
				MarriageIndex: uint32(i + 1),
				NumUnits:      atx.TotalNumUnits(),
				PrevATXIndex:  uint32(i + 1),
			}
			totalNumUnits += post.NumUnits
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}
		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = previousATXs
		merged.Sign(sig)

		atxHandler.expectMergedAtxV2(merged, equivocationSet, []uint64{poetLeaves})
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)

		atx, err := atxs.Get(atxHandler.cdb, merged.ID())
		require.NoError(t, err)
		require.Equal(t, totalNumUnits, atx.NumUnits)
		require.Equal(t, sig.NodeID(), atx.SmesherID)
		require.EqualValues(t, totalNumUnits*poetLeaves/tickSize, atx.Weight)
	})
	t.Run("merged IDs on 4 poets", func(t *testing.T) {
		const tickSize = 33
		atxHandler := newV2TestHandler(t, golden)
		atxHandler.tickSize = tickSize

		// Marry IDs
		mATX, otherATXs := marryIDs(t, atxHandler, signers, golden)
		previousATXs := []types.ATXID{mATX.ID()}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX
		merged := &wire.ActivationTxV2{
			PublishEpoch:   mATX.PublishEpoch + 2,
			PreviousATXs:   previousATXs,
			PositioningATX: mATX.ID(),
			Coinbase:       types.GenerateAddress([]byte("aaaa")),
			VRFNonce:       uint64(999),
			NIPosts:        make([]wire.NIPostV2, 4),
		}
		atxsPerPoet := [][]*wire.ActivationTxV2{
			append([]*wire.ActivationTxV2{mATX}, otherATXs[0]),
			otherATXs[1:2],
			otherATXs[2:3],
			otherATXs[3:4],
		}
		var totalNumUnits uint32
		unitsPerPoet := make([]uint32, 4)
		var idx uint32
		for nipostId := range 4 {
			for _, atx := range atxsPerPoet[nipostId] {
				post := wire.SubPostV2{
					MarriageIndex: idx,
					NumUnits:      atx.TotalNumUnits(),
					PrevATXIndex:  idx,
				}
				unitsPerPoet[nipostId] += post.NumUnits
				totalNumUnits += post.NumUnits
				merged.NIPosts[nipostId].Posts = append(merged.NIPosts[nipostId].Posts, post)
				idx++
			}
		}

		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = previousATXs
		merged.Sign(sig)

		poetLeaves := []uint64{100, 500, 70, 200}
		minPoetLeaves := slices.Min(poetLeaves)

		atxHandler.expectMergedAtxV2(merged, equivocationSet, poetLeaves)
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)

		marriageATX, err := atxs.Get(atxHandler.cdb, mATX.ID())
		require.NoError(t, err)
		atx, err := atxs.Get(atxHandler.cdb, merged.ID())
		require.NoError(t, err)
		require.Equal(t, totalNumUnits, atx.NumUnits)
		require.Equal(t, sig.NodeID(), atx.SmesherID)
		require.Equal(t, minPoetLeaves/tickSize, atx.TickCount)
		require.Equal(t, marriageATX.TickHeight()+atx.TickCount, atx.TickHeight())
		// the total weight is summed weight on each poet
		var weight uint64
		for i, units := range unitsPerPoet {
			ticks := poetLeaves[i] / tickSize
			weight += uint64(units) * ticks
		}
		require.EqualValues(t, weight, atx.Weight)
	})
	t.Run("signer must be included merged ATX", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		// Marry IDs
		mATX, otherATXs := marryIDs(t, atxHandler, signers, golden)
		previousATXs := []types.ATXID{}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		merged.NIPosts[0].Posts = []wire.SubPostV2{} // remove signer's PoST
		for i, atx := range otherATXs {
			post := wire.SubPostV2{
				MarriageIndex: uint32(i + 1),
				NumUnits:      atx.TotalNumUnits(),
				PrevATXIndex:  uint32(i),
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}
		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = previousATXs
		merged.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer().Return(merged.PublishEpoch.FirstLayer())
		atxHandler.expectFetchDeps(merged)
		atxHandler.expectVerifyNIPoSTs(merged, equivocationSet, []uint64{200})

		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.ErrorContains(t, err, "ATX signer not present in merged ATX")
		require.ErrorIs(t, err, pubsub.ErrValidationReject)
	})
	t.Run("ID must be present max 1 times", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		// Marry IDs
		mATX, otherATXs := marryIDs(t, atxHandler, signers[:2], golden)
		previousATXs := []types.ATXID{mATX.ID()}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		// Insert the same ID twice
		for range 2 {
			post := wire.SubPostV2{
				MarriageIndex: 1,
				PrevATXIndex:  1,
				NumUnits:      otherATXs[0].TotalNumUnits(),
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}
		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = previousATXs
		merged.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer().Return(merged.PublishEpoch.FirstLayer())
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.ErrorContains(t, err, "ID present twice (duplicated marriage index)")
		require.ErrorIs(t, err, pubsub.ErrValidationReject)
	})
	t.Run("ID must use previous ATX containing itself", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		// Marry IDs
		mATX, otherATXs := marryIDs(t, atxHandler, signers[:2], golden)
		previousATXs := []types.ATXID{mATX.ID()}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		post := wire.SubPostV2{
			MarriageIndex: 1,
			PrevATXIndex:  0, // use wrong previous ATX
			NumUnits:      otherATXs[0].TotalNumUnits(),
		}
		merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)

		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = previousATXs
		merged.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer().Return(merged.PublishEpoch.FirstLayer())
		atxHandler.expectFetchDeps(merged)
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.ErrorIs(t, err, pubsub.ErrValidationReject)
	})
	t.Run("previous checkpointed ATX must include every ID", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		// Marry IDs
		mATX, _ := marryIDs(t, atxHandler, signers, golden)

		prev := atxs.CheckpointAtx{
			Epoch:         mATX.PublishEpoch + 1,
			ID:            types.RandomATXID(),
			CommitmentATX: types.RandomATXID(),
			SmesherID:     sig.NodeID(),
			NumUnits:      10,
			Units:         make(map[types.NodeID]uint32),
		}
		for _, id := range equivocationSet {
			prev.Units[id] = 10
		}
		require.NoError(t, atxs.AddCheckpointed(atxHandler.cdb, &prev))

		// Process a merged ATX
		merged := newSoloATXv2(t, prev.Epoch+1, prev.ID, golden)
		merged.NIPosts[0].Posts = []wire.SubPostV2{}
		for marriageIdx := range equivocationSet {
			post := wire.SubPostV2{
				MarriageIndex: uint32(marriageIdx),
				NumUnits:      7,
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}

		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID
		merged.Sign(sig)

		atxHandler.expectMergedAtxV2(merged, equivocationSet, []uint64{100})
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)

		// checkpoint again but not include one of the IDs
		prev.ID = types.RandomATXID()
		prev.Epoch = merged.PublishEpoch + 1
		clear(prev.Units)
		for _, id := range equivocationSet[:1] {
			prev.Units[id] = 10
		}
		require.NoError(t, atxs.AddCheckpointed(atxHandler.cdb, &prev))

		merged = newSoloATXv2(t, prev.Epoch+1, prev.ID, golden)
		merged.NIPosts[0].Posts = []wire.SubPostV2{}
		for marriageIdx := range equivocationSet {
			post := wire.SubPostV2{
				MarriageIndex: uint32(marriageIdx),
				NumUnits:      7,
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}
		merged.MarriageATX = &mATXID
		merged.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer().Return(merged.PublishEpoch.FirstLayer())
		atxHandler.expectFetchDeps(merged)
		err = atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.ErrorIs(t, err, pubsub.ErrValidationReject)
	})
	t.Run("publishing two merged ATXs from one marriage set is malfeasance", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		// Marry 4 IDs
		mATX, otherATXs := marryIDs(t, atxHandler, signers, golden)
		previousATXs := []types.ATXID{mATX.ID()}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX for 2 IDs
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		merged.NIPosts[0].Posts = []wire.SubPostV2{}
		for i := range equivocationSet[:2] {
			post := wire.SubPostV2{
				MarriageIndex: uint32(i),
				PrevATXIndex:  uint32(i),
				NumUnits:      4,
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}

		mATXID := mATX.ID()

		merged.MarriageATX = &mATXID
		merged.PreviousATXs = []types.ATXID{mATX.ID(), otherATXs[0].ID()}
		merged.Sign(sig)

		atxHandler.expectMergedAtxV2(merged, equivocationSet, []uint64{100})
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)

		// Process a second merged ATX for the same equivocation set, but different IDs
		merged = newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		merged.NIPosts[0].Posts = []wire.SubPostV2{}
		for i := range equivocationSet[:2] {
			post := wire.SubPostV2{
				MarriageIndex: uint32(i + 2),
				PrevATXIndex:  uint32(i),
				NumUnits:      4,
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}

		mATXID = mATX.ID()
		merged.MarriageATX = &mATXID
		merged.PreviousATXs = []types.ATXID{otherATXs[1].ID(), otherATXs[2].ID()}
		merged.Sign(signers[2])

		verifier := wire.NewMockMalfeasanceValidator(atxHandler.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHandler.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		atxHandler.expectMergedAtxV2(merged, equivocationSet, []uint64{100})
		atxHandler.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			merged.SmesherID,
			gomock.AssignableToTypeOf(&wire.ProofDoubleMerge{}),
		).DoAndReturn(func(ctx context.Context, id types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofDoubleMerge)
			nId, err := malProof.Valid(context.Background(), verifier)
			require.NoError(t, err)
			require.Equal(t, merged.SmesherID, nId)
			return nil
		})
		err = atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)
	})
	t.Run("publishing two merged ATXs (one checkpointed)", func(t *testing.T) {
		t.Skip("syntactically validating double merge where one ATX is checkpointed isn't implemented yet")
		atxHandler := newV2TestHandler(t, golden)

		mATX, otherATXs := marryIDs(t, atxHandler, signers, golden)
		mATXID := mATX.ID()

		// Insert checkpointed merged ATX
		checkpointedATX := &atxs.CheckpointAtx{
			Epoch:       mATX.PublishEpoch + 2,
			ID:          types.RandomATXID(),
			SmesherID:   signers[0].NodeID(),
			MarriageATX: &mATXID,
		}
		require.NoError(t, atxs.AddCheckpointed(atxHandler.cdb, checkpointedATX))

		// create and process another merged ATX
		merged := newSoloATXv2(t, checkpointedATX.Epoch, mATX.ID(), golden)
		merged.NIPosts[0].Posts = []wire.SubPostV2{}
		for i := range equivocationSet[2:] {
			post := wire.SubPostV2{
				MarriageIndex: uint32(i + 2),
				PrevATXIndex:  uint32(i),
				NumUnits:      4,
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}

		merged.MarriageATX = &mATXID
		merged.PreviousATXs = []types.ATXID{otherATXs[1].ID(), otherATXs[2].ID(), otherATXs[3].ID()}
		merged.Sign(signers[2])

		// This is syntactically invalid as all nodes in the network
		// should already have the checkpointed merged ATX.
		atxHandler.expectMergedAtxV2(merged, equivocationSet, []uint64{100})
		err := atxHandler.processATX(context.Background(), "", merged, time.Now())
		require.Error(t, err)
	})
}

func TestCollectDeps_AtxV2(t *testing.T) {
	goldenATX := types.RandomATXID()
	prev0 := types.RandomATXID()
	prev1 := types.RandomATXID()
	positioning := types.RandomATXID()
	commitment := types.RandomATXID()
	marriage := types.RandomATXID()
	ref0 := types.RandomATXID()
	ref1 := types.RandomATXID()
	poetA := types.RandomHash()
	poetB := types.RandomHash()

	atxHandler := newV2TestHandler(t, goldenATX)

	t.Run("all unique deps", func(t *testing.T) {
		t.Parallel()
		atx := wire.ActivationTxV2{
			PreviousATXs:   []types.ATXID{prev0, prev1},
			PositioningATX: positioning,
			Initial:        &wire.InitialAtxPartsV2{CommitmentATX: commitment},
			MarriageATX:    &marriage,
			NIPosts: []wire.NIPostV2{
				{Challenge: poetA},
				{Challenge: poetB},
			},
			Marriages: []wire.MarriageCertificate{
				{ReferenceAtx: types.EmptyATXID},
				{ReferenceAtx: ref0},
				{ReferenceAtx: ref1},
			},
		}
		poetDeps, atxIDs := atxHandler.collectAtxDeps(&atx)
		require.ElementsMatch(t, []types.Hash32{poetA, poetB}, poetDeps)
		require.ElementsMatch(t, []types.ATXID{prev0, prev1, positioning, commitment, marriage, ref0, ref1}, atxIDs)
	})
	t.Run("eliminates duplicates", func(t *testing.T) {
		t.Parallel()
		atxA := types.RandomATXID()
		atx := wire.ActivationTxV2{
			PreviousATXs:   []types.ATXID{atxA, atxA},
			PositioningATX: atxA,
			Initial:        &wire.InitialAtxPartsV2{CommitmentATX: atxA},
			MarriageATX:    &atxA,
			NIPosts: []wire.NIPostV2{
				{Challenge: poetA},
				{Challenge: poetA},
			},
		}
		poetDeps, atxIDs := atxHandler.collectAtxDeps(&atx)
		require.ElementsMatch(t, []types.Hash32{poetA}, poetDeps)
		require.ElementsMatch(t, []types.ATXID{atxA}, atxIDs)
	})
	t.Run("nil commitment ATX", func(t *testing.T) {
		t.Parallel()
		atx := wire.ActivationTxV2{
			PreviousATXs:   []types.ATXID{prev0, prev1},
			PositioningATX: positioning,
			MarriageATX:    &marriage,
			NIPosts: []wire.NIPostV2{
				{Challenge: poetA},
				{Challenge: poetB},
			},
		}
		poetDeps, atxIDs := atxHandler.collectAtxDeps(&atx)
		require.ElementsMatch(t, []types.Hash32{poetA, poetB}, poetDeps)
		require.ElementsMatch(t, []types.ATXID{prev0, prev1, positioning, marriage}, atxIDs)
	})
	t.Run("filters out golden ATX and empty ATX", func(t *testing.T) {
		t.Parallel()
		atx := wire.ActivationTxV2{
			PreviousATXs:   []types.ATXID{types.EmptyATXID, goldenATX},
			Initial:        &wire.InitialAtxPartsV2{CommitmentATX: goldenATX},
			PositioningATX: goldenATX,
		}
		poetDeps, atxIDs := atxHandler.collectAtxDeps(&atx)
		require.Empty(t, poetDeps)
		require.Empty(t, atxIDs)
	})
}

func TestHandlerV2_RegisterReferences(t *testing.T) {
	atxHdlr := newV2TestHandler(t, types.RandomATXID())

	poets := []types.Hash32{types.RandomHash(), types.RandomHash()}
	atxs := []types.ATXID{types.RandomATXID(), types.RandomATXID()}
	expectedHashes := poets
	for _, atx := range atxs {
		expectedHashes = append(expectedHashes, atx.Hash32())
	}

	atxHdlr.mockFetch.EXPECT().RegisterPeerHashes(atxHdlr.local, gomock.InAnyOrder(expectedHashes))
	atxHdlr.registerHashes(atxHdlr.local, poets, atxs)
}

func TestHandlerV2_FetchesReferences(t *testing.T) {
	golden := types.RandomATXID()
	t.Run("fetch poet and atxs", func(t *testing.T) {
		t.Parallel()
		atxHdlr := newV2TestHandler(t, golden)

		poets := []types.Hash32{types.RandomHash(), types.RandomHash()}
		atxs := []types.ATXID{types.RandomATXID(), types.RandomATXID()}

		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[0])
		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[1])
		atxHdlr.mockFetch.EXPECT().GetAtxs(gomock.Any(), atxs, gomock.Any())
		require.NoError(t, atxHdlr.fetchReferences(context.Background(), poets, atxs))
	})

	t.Run("failed to fetch poet proof", func(t *testing.T) {
		t.Parallel()
		atxHdlr := newV2TestHandler(t, golden)

		poets := []types.Hash32{types.RandomHash(), types.RandomHash()}

		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[0])
		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[1]).Return(errors.New("pooh"))
		require.Error(t, atxHdlr.fetchReferences(context.Background(), poets, nil))
	})
	t.Run("reject ATX when dependency poet proof is rejected", func(t *testing.T) {
		t.Parallel()
		atxHdlr := newV2TestHandler(t, golden)

		poets := []types.Hash32{types.RandomHash()}
		atxs := []types.ATXID{types.RandomATXID()}
		var batchErr fetch.BatchError
		batchErr.Add(atxs[0].Hash32(), pubsub.ErrValidationReject)

		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[0]).Return(&batchErr)
		atxHdlr.mockFetch.EXPECT().GetAtxs(gomock.Any(), atxs, gomock.Any())

		require.ErrorIs(t, atxHdlr.fetchReferences(context.Background(), poets, atxs), pubsub.ErrValidationReject)
	})

	t.Run("failed to fetch atxs", func(t *testing.T) {
		t.Parallel()
		atxHdlr := newV2TestHandler(t, golden)

		poets := []types.Hash32{types.RandomHash(), types.RandomHash()}
		atxs := []types.ATXID{types.RandomATXID(), types.RandomATXID()}

		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[0])
		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[1])
		atxHdlr.mockFetch.EXPECT().GetAtxs(gomock.Any(), atxs, gomock.Any()).Return(errors.New("oh"))
		require.Error(t, atxHdlr.fetchReferences(context.Background(), poets, atxs))
	})
	t.Run("reject ATX when dependency ATX is rejected", func(t *testing.T) {
		t.Parallel()
		atxHdlr := newV2TestHandler(t, golden)

		poets := []types.Hash32{types.RandomHash()}
		atxs := []types.ATXID{types.RandomATXID(), types.RandomATXID()}
		var batchErr fetch.BatchError
		batchErr.Add(atxs[0].Hash32(), pubsub.ErrValidationReject)

		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[0])
		atxHdlr.mockFetch.EXPECT().GetAtxs(gomock.Any(), atxs, gomock.Any()).Return(&batchErr)

		require.ErrorIs(t, atxHdlr.fetchReferences(context.Background(), poets, atxs), pubsub.ErrValidationReject)
	})
	t.Run("no atxs to fetch", func(t *testing.T) {
		t.Parallel()
		atxHdlr := newV2TestHandler(t, golden)

		poets := []types.Hash32{types.RandomHash()}

		atxHdlr.mockFetch.EXPECT().GetPoetProof(gomock.Any(), poets[0])
		require.NoError(t, atxHdlr.fetchReferences(context.Background(), poets, nil))
	})
}

func Test_ValidatePositioningAtx(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()

	t.Run("not found", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)
		_, err := atxHandler.validatePositioningAtx(1, golden, types.RandomATXID())
		require.ErrorContains(t, err, "not found")
	})
	t.Run("golden", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		height, err := atxHandler.validatePositioningAtx(0, golden, golden)
		require.NoError(t, err)
		require.Zero(t, height)
	})
	t.Run("non-golden", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		positioningAtx := &types.ActivationTx{
			BaseTickHeight: 100,
		}
		positioningAtx.SetID(types.RandomATXID())
		atxs.Add(atxHandler.cdb, positioningAtx, types.AtxBlob{})

		height, err := atxHandler.validatePositioningAtx(1, golden, positioningAtx.ID())
		require.NoError(t, err)
		require.Equal(t, positioningAtx.TickHeight(), height)
	})
	t.Run("reject pos ATX from the same epoch", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		positioningAtx := &types.ActivationTx{
			PublishEpoch: 1,
		}
		positioningAtx.SetID(types.RandomATXID())
		atxs.Add(atxHandler.cdb, positioningAtx, types.AtxBlob{})

		_, err := atxHandler.validatePositioningAtx(1, golden, positioningAtx.ID())
		require.Error(t, err)
	})
	t.Run("reject pos ATX from a future epoch", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		positioningAtx := &types.ActivationTx{
			PublishEpoch: 2,
		}
		positioningAtx.SetID(types.RandomATXID())
		atxs.Add(atxHandler.cdb, positioningAtx, types.AtxBlob{})

		_, err := atxHandler.validatePositioningAtx(1, golden, positioningAtx.ID())
		require.Error(t, err)
	})
}

func Test_ValidateMarriages(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	t.Run("marriage ATX not set (solo ATX)", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		set, err := atxHandler.equivocationSet(atx)
		require.NoError(t, err)
		require.Equal(t, []types.NodeID{atx.SmesherID}, set)
	})
	t.Run("smesher is not married", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		atx := newSoloATXv2(t, 0, types.RandomATXID(), golden)
		atx.MarriageATX = &golden
		atx.Sign(sig)

		_, err := atxHandler.equivocationSet(atx)
		require.ErrorContains(t, err, "smesher is not married")
	})
	t.Run("marriage ATX must be published 2 epochs prior merging IDs", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)

		marriageATX, _ := marryIDs(t, atxHandler, []*signing.EdSigner{sig, otherSig}, golden)

		atx := newSoloATXv2(t, marriageATX.PublishEpoch+1, types.RandomATXID(), golden)
		marriageATXID := marriageATX.ID()
		atx.MarriageATX = &marriageATXID
		atx.Sign(sig)

		_, err = atxHandler.equivocationSet(atx)
		require.ErrorContains(t, err, "marriage atx must be published at least 2 epochs before")
	})
	t.Run("can't use somebody else's marriage ATX", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)

		marriageATX, _ := marryIDs(t, atxHandler, []*signing.EdSigner{sig, otherSig}, golden)

		atx := newSoloATXv2(t, marriageATX.PublishEpoch+1, types.RandomATXID(), golden)
		marriageATXID := types.RandomATXID()
		atx.MarriageATX = &marriageATXID
		atx.Sign(sig)

		_, err = atxHandler.equivocationSet(atx)
		require.ErrorContains(t, err, "smesher's marriage ATX ID mismatch")
	})
	t.Run("smesher is married", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		marriage := newInitialATXv2(t, golden)
		marriage.Marriages = []wire.MarriageCertificate{{
			Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
		}}

		var otherIds []marriedId
		for range 5 {
			signer, err := signing.NewEdSigner()
			require.NoError(t, err)
			atx := atxHandler.createAndProcessInitial(signer)
			otherIds = append(otherIds, marriedId{signer, atx})
		}

		expectedSet := []types.NodeID{sig.NodeID()}

		for _, id := range otherIds {
			cert := wire.MarriageCertificate{
				ReferenceAtx: id.refAtx.ID(),
				Signature:    id.signer.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			}
			marriage.Marriages = append(marriage.Marriages, cert)
			expectedSet = append(expectedSet, id.signer.NodeID())
		}
		marriage.Sign(sig)

		err := atxHandler.processInitial(marriage)
		require.NoError(t, err)

		atx := newSoloATXv2(t, 0, marriage.ID(), golden)
		atx.PublishEpoch = marriage.PublishEpoch + 2
		marriageATXID := marriage.ID()
		atx.MarriageATX = &marriageATXID
		atx.Sign(sig)

		set, err := atxHandler.equivocationSet(atx)
		require.NoError(t, err)
		require.ElementsMatch(t, expectedSet, set)
	})
}

func Test_ValidateCommitmentAtx(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()

	t.Run("golden is fine", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		err := atxHandler.validateCommitmentAtx(golden, golden, 0)
		require.NoError(t, err)
	})
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		commitment := &types.ActivationTx{PublishEpoch: 3}
		commitment.SetID(types.RandomATXID())
		require.NoError(t, atxs.Add(atxHandler.cdb, commitment, types.AtxBlob{}))
		err := atxHandler.validateCommitmentAtx(golden, commitment.ID(), 4)
		require.NoError(t, err)
	})
	t.Run("too new", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)
		commitment := &types.ActivationTx{PublishEpoch: 3}
		commitment.SetID(types.RandomATXID())
		require.NoError(t, atxs.Add(atxHandler.cdb, commitment, types.AtxBlob{}))
		err := atxHandler.validateCommitmentAtx(golden, commitment.ID(), 3)
		require.ErrorContains(t, err, "must be after commitment atx")
		err = atxHandler.validateCommitmentAtx(golden, commitment.ID(), 2)
		require.ErrorContains(t, err, "must be after commitment atx")
	})
	t.Run("not found", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)
		err := atxHandler.validateCommitmentAtx(golden, types.RandomATXID(), 3)
		require.ErrorContains(t, err, "not found")
	})
}

func Test_ValidatePreviousATX(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	atxHandler := newV2TestHandler(t, golden)
	t.Run("out of range", func(t *testing.T) {
		t.Parallel()
		post := &wire.SubPostV2{
			PrevATXIndex: 1,
		}
		_, err := atxHandler.validatePreviousAtx(types.RandomNodeID(), post, nil)
		require.ErrorContains(t, err, "out of bounds")
	})
	t.Run("smesher ID not present", func(t *testing.T) {
		t.Parallel()
		prev := &types.ActivationTx{}
		prev.SetID(types.RandomATXID())
		require.NoError(t, atxs.SetPost(atxHandler.cdb, prev.ID(), types.EmptyATXID, 0, types.RandomNodeID(), 13, 0))

		_, err := atxHandler.validatePreviousAtx(types.RandomNodeID(), &wire.SubPostV2{}, []*types.ActivationTx{prev})
		require.Error(t, err)
	})
	t.Run("effective units is min(previous, atx) for given smesher", func(t *testing.T) {
		t.Parallel()
		id := types.RandomNodeID()
		other := types.RandomNodeID()
		prev := &types.ActivationTx{}
		prev.SetID(types.RandomATXID())
		require.NoError(t, atxs.SetPost(atxHandler.cdb, prev.ID(), types.EmptyATXID, 0, id, 7, 0))
		require.NoError(t, atxs.SetPost(atxHandler.cdb, prev.ID(), types.EmptyATXID, 0, other, 13, 0))

		units, err := atxHandler.validatePreviousAtx(id, &wire.SubPostV2{NumUnits: 100}, []*types.ActivationTx{prev})
		require.NoError(t, err)
		require.EqualValues(t, 7, units)

		units, err = atxHandler.validatePreviousAtx(other, &wire.SubPostV2{NumUnits: 100}, []*types.ActivationTx{prev})
		require.NoError(t, err)
		require.EqualValues(t, 13, units)

		units, err = atxHandler.validatePreviousAtx(id, &wire.SubPostV2{NumUnits: 2}, []*types.ActivationTx{prev})
		require.NoError(t, err)
		require.EqualValues(t, 2, units)
	})
	t.Run("previous merged, doesn't contain ID", func(t *testing.T) {
		t.Parallel()
		id := types.RandomNodeID()
		other := types.RandomNodeID()
		prev := &types.ActivationTx{}
		prev.SetID(types.RandomATXID())
		require.NoError(t, atxs.SetPost(atxHandler.cdb, prev.ID(), types.EmptyATXID, 0, other, 13, 0))

		_, err := atxHandler.validatePreviousAtx(id, &wire.SubPostV2{NumUnits: 100}, []*types.ActivationTx{prev})
		require.Error(t, err)
	})
}

func TestHandlerV2_SyntacticallyValidateDeps(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	t.Run("invalid commitment ATX in initial ATX", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		atx := newInitialATXv2(t, golden)
		atx.Initial.CommitmentATX = types.RandomATXID()
		atx.Sign(sig)

		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		require.ErrorContains(t, err, "verifying commitment ATX")
	})
	t.Run("can't find previous ATX", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		atx := newSoloATXv2(t, 0, types.RandomATXID(), golden)
		atx.Sign(sig)

		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		require.ErrorContains(t, err, "fetching previous atx")
	})
	t.Run("previous ATX too new", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		prev := atxHandler.createAndProcessInitial(sig)

		atx := newSoloATXv2(t, 0, prev.ID(), golden)
		atx.Sign(sig)

		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		require.ErrorContains(t, err, "previous atx is too new")
	})
	t.Run("previous ATX by different smesher", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		prev := atxHandler.createAndProcessInitial(otherSig)

		atx := newSoloATXv2(t, 2, prev.ID(), golden)
		atx.Sign(sig)

		_, err = atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		require.Error(t, err)
	})
	t.Run("invalid PoST", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler.mValidator.EXPECT().PoetMembership(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		atxHandler.mValidator.EXPECT().
			PostV2(
				gomock.Any(),
				sig.NodeID(),
				golden,
				wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
				atx.NIPosts[0].Challenge.Bytes(),
				atx.TotalNumUnits(),
				gomock.Any(),
			).
			Return(errors.New("post failure"))
		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		require.ErrorContains(t, err, "post failure")
	})
	t.Run("invalid PoST index initial ATX - generates a malfeasance proof", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler.mValidator.EXPECT().PoetMembership(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		atxHandler.mValidator.EXPECT().PostV2(
			context.Background(),
			atx.SmesherID,
			atx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			gomock.Cond(func(opt validatorOption) bool {
				opts := &validatorOptions{}
				opt(opts)
				return opts.postSubsetSeed != nil
			}),
		).Return(&verifying.ErrInvalidIndex{Index: 7})

		for invalidPostIdx := 0; invalidPostIdx < 10; invalidPostIdx++ {
			atxHandler.mValidator.EXPECT().PostV2(
				context.Background(),
				atx.SmesherID,
				atx.Initial.CommitmentATX,
				wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
				atx.NIPosts[0].Challenge.Bytes(),
				atx.TotalNumUnits(),
				gomock.Cond(func(opt validatorOption) bool {
					opts := &validatorOptions{}
					opt(opts)
					return opts.postIdx != nil && *opts.postIdx == invalidPostIdx
				}),
			).Return(&verifying.ErrInvalidIndex{Index: invalidPostIdx})
		}

		atxHandler.mValidator.EXPECT().PostV2(
			context.Background(),
			atx.SmesherID,
			atx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			gomock.Cond(func(opt validatorOption) bool {
				opts := &validatorOptions{}
				opt(opts)
				return opts.postIdx != nil && *opts.postIdx == 10
			}),
		).Return(nil)

		verifier := wire.NewMockMalfeasanceValidator(atxHandler.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHandler.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		verifier.EXPECT().PostIndex(
			context.Background(),
			atx.SmesherID,
			atx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			10,
		).Return(nil)

		verifier.EXPECT().PostIndex(
			context.Background(),
			atx.SmesherID,
			atx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			7,
		).Return(errors.New("invalid post index"))

		atxHandler.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			sig.NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofInvalidPost{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofInvalidPost)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, sig.NodeID(), nId)
			return nil
		})
		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		vErr := &verifying.ErrInvalidIndex{}
		require.ErrorAs(t, err, &vErr)
		require.Equal(t, 7, vErr.Index)
	})
	t.Run("invalid PoST index solo ATX - generates a malfeasance proof", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		initialAtx := atxHandler.createAndProcessInitial(sig)

		atx := newSoloATXv2(t, initialAtx.PublishEpoch+1, initialAtx.ID(), initialAtx.ID())
		atx.Sign(sig)

		atxHandler.mValidator.EXPECT().PoetMembership(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		atxHandler.mValidator.EXPECT().PostV2(
			context.Background(),
			atx.SmesherID,
			initialAtx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			gomock.Cond(func(opt validatorOption) bool {
				opts := &validatorOptions{}
				opt(opts)
				return opts.postSubsetSeed != nil
			}),
		).Return(&verifying.ErrInvalidIndex{Index: 7})

		for invalidPostIdx := 0; invalidPostIdx < 10; invalidPostIdx++ {
			atxHandler.mValidator.EXPECT().PostV2(
				context.Background(),
				atx.SmesherID,
				initialAtx.Initial.CommitmentATX,
				wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
				atx.NIPosts[0].Challenge.Bytes(),
				atx.TotalNumUnits(),
				gomock.Cond(func(opt validatorOption) bool {
					opts := &validatorOptions{}
					opt(opts)
					return opts.postIdx != nil && *opts.postIdx == invalidPostIdx
				}),
			).Return(&verifying.ErrInvalidIndex{Index: invalidPostIdx})
		}

		atxHandler.mValidator.EXPECT().PostV2(
			context.Background(),
			atx.SmesherID,
			initialAtx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			gomock.Cond(func(opt validatorOption) bool {
				opts := &validatorOptions{}
				opt(opts)
				return opts.postIdx != nil && *opts.postIdx == 10
			}),
		).Return(nil)

		verifier := wire.NewMockMalfeasanceValidator(atxHandler.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHandler.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		verifier.EXPECT().PostIndex(
			context.Background(),
			atx.SmesherID,
			initialAtx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			10,
		).Return(nil)

		verifier.EXPECT().PostIndex(
			context.Background(),
			atx.SmesherID,
			initialAtx.Initial.CommitmentATX,
			wire.PostFromWireV1(&atx.NIPosts[0].Posts[0].Post),
			atx.NIPosts[0].Challenge.Bytes(),
			atx.TotalNumUnits(),
			7,
		).Return(errors.New("invalid post index"))

		atxHandler.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			sig.NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofInvalidPost{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofInvalidPost)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, sig.NodeID(), nId)
			return nil
		})
		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		vErr := &verifying.ErrInvalidIndex{}
		require.ErrorAs(t, err, &vErr)
		require.Equal(t, 7, vErr.Index)
	})
	t.Run("invalid PoST index merged ATX - generates a malfeasance proof", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		marrySig, err := signing.NewEdSigner()
		require.NoError(t, err)
		pubSig, err := signing.NewEdSigner()
		require.NoError(t, err)

		// Marry IDs
		mATX, otherATXs := marryIDs(t, atxHandler, []*signing.EdSigner{marrySig, sig, pubSig}, golden)
		previousATXs := []types.ATXID{mATX.ID()}
		for _, atx := range otherATXs {
			previousATXs = append(previousATXs, atx.ID())
		}

		// Process a merged ATX
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		for i, atx := range otherATXs {
			post := wire.SubPostV2{
				MarriageIndex: uint32(i + 1),
				NumUnits:      atx.TotalNumUnits(),
				PrevATXIndex:  uint32(i + 1),
			}
			merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		}
		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = previousATXs
		merged.Sign(sig)

		equivocationSet := []types.NodeID{marrySig.NodeID(), sig.NodeID(), pubSig.NodeID()}
		atxHandler.mValidator.EXPECT().PoetMembership(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		for _, post := range merged.NIPosts[0].Posts {
			call := atxHandler.mValidator.EXPECT().PostV2(
				context.Background(),
				equivocationSet[post.MarriageIndex],
				gomock.Any(),
				wire.PostFromWireV1(&post.Post),
				merged.NIPosts[0].Challenge.Bytes(),
				post.NumUnits,
				gomock.Any(),
			)
			if equivocationSet[post.MarriageIndex] == sig.NodeID() {
				call.Return(&verifying.ErrInvalidIndex{Index: 7})
			} else {
				call.AnyTimes()
			}
		}

		for invalidPostIdx := 0; invalidPostIdx < 10; invalidPostIdx++ {
			atxHandler.mValidator.EXPECT().PostV2(
				context.Background(),
				sig.NodeID(),
				gomock.Any(),
				gomock.Any(),
				merged.NIPosts[0].Challenge.Bytes(),
				gomock.Any(),
				gomock.Cond(func(opt validatorOption) bool {
					opts := &validatorOptions{}
					opt(opts)
					return opts.postIdx != nil && *opts.postIdx == invalidPostIdx
				}),
			).Return(&verifying.ErrInvalidIndex{Index: invalidPostIdx})
		}

		atxHandler.mValidator.EXPECT().PostV2(
			context.Background(),
			sig.NodeID(),
			gomock.Any(),
			gomock.Any(),
			merged.NIPosts[0].Challenge.Bytes(),
			gomock.Any(),
			gomock.Cond(func(opt validatorOption) bool {
				opts := &validatorOptions{}
				opt(opts)
				return opts.postIdx != nil && *opts.postIdx == 10
			}),
		).Return(nil)

		verifier := wire.NewMockMalfeasanceValidator(atxHandler.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHandler.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		verifier.EXPECT().PostIndex(
			context.Background(),
			sig.NodeID(),
			gomock.Any(),
			gomock.Any(),
			merged.NIPosts[0].Challenge.Bytes(),
			gomock.Any(),
			10,
		).Return(nil)

		verifier.EXPECT().PostIndex(
			context.Background(),
			sig.NodeID(),
			gomock.Any(),
			gomock.Any(),
			merged.NIPosts[0].Challenge.Bytes(),
			gomock.Any(),
			7,
		).Return(errors.New("invalid post index"))

		atxHandler.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			sig.NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofInvalidPost{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofInvalidPost)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, sig.NodeID(), nId)
			return nil
		})
		_, err = atxHandler.syntacticallyValidateDeps(context.Background(), merged)
		vErr := &verifying.ErrInvalidIndex{}
		require.ErrorAs(t, err, &vErr)
		require.Equal(t, 7, vErr.Index)
	})
	t.Run("invalid PoET membership proof", func(t *testing.T) {
		atxHandler := newV2TestHandler(t, golden)

		atx := newInitialATXv2(t, golden)
		atx.Sign(sig)

		atxHandler.mValidator.EXPECT().
			PoetMembership(gomock.Any(), gomock.Any(), atx.NIPosts[0].Challenge, gomock.Any()).
			Return(0, errors.New("poet failure"))
		_, err := atxHandler.syntacticallyValidateDeps(context.Background(), atx)
		require.ErrorContains(t, err, "poet failure")
	})
}

func Test_Marriages(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)
	t.Run("invalid marriage signature", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		atx := newInitialATXv2(t, golden)
		atx.Marriages = []wire.MarriageCertificate{{
			Signature: types.RandomEdSignature(),
		}}

		_, err = atxHandler.validateMarriages(atx)
		require.ErrorContains(t, err, "invalid marriage[0] signature")
	})
	t.Run("valid marriage", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		atx, _ := marryIDs(t, atxHandler, []*signing.EdSigner{sig, otherSig}, golden)

		info, err := marriage.FindByNodeID(atxHandler.cdb, sig.NodeID())
		require.NoError(t, err)
		require.Equal(t, atx.ID(), info.ATX)

		info, err = marriage.FindByNodeID(atxHandler.cdb, otherSig.NodeID())
		require.NoError(t, err)
		require.Equal(t, atx.ID(), info.ATX)

		id, err := marriage.FindIDByNodeID(atxHandler.cdb, sig.NodeID())
		require.NoError(t, err)
		set, err := marriage.NodeIDsByID(atxHandler.cdb, id)
		require.NoError(t, err)
		require.ElementsMatch(t, []types.NodeID{sig.NodeID(), otherSig.NodeID()}, set)
	})
	t.Run("can't marry twice in the same marriage ATX", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		othersAtx := atxHandler.createAndProcessInitial(otherSig)

		othersSecondAtx := newSoloATXv2(t, othersAtx.PublishEpoch+1, othersAtx.ID(), othersAtx.ID())
		othersSecondAtx.Sign(otherSig)
		err = atxHandler.processSoloAtx(othersSecondAtx)
		require.NoError(t, err)

		atx := newInitialATXv2(t, golden)
		atx.Marriages = []wire.MarriageCertificate{
			{
				Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
			{
				ReferenceAtx: othersAtx.ID(),
				Signature:    otherSig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
			{
				ReferenceAtx: othersSecondAtx.ID(),
				Signature:    otherSig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
		}
		atx.Sign(sig)

		_, err = atxHandler.validateMarriages(atx)
		require.ErrorContains(t, err, "more than 1 marriage certificate for ID")
	})
	t.Run("can't marry twice (separate marriages)", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		atx, _ := marryIDs(t, atxHandler, []*signing.EdSigner{sig, otherSig}, golden)

		// otherSig2 cannot marry sig, trying to extend its set.
		otherSig2, err := signing.NewEdSigner()
		require.NoError(t, err)
		others2Atx := atxHandler.createAndProcessInitial(otherSig2)
		atx2 := newSoloATXv2(t, atx.PublishEpoch+1, atx.ID(), atx.ID())
		atx2.Marriages = []wire.MarriageCertificate{
			{
				Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
			{
				ReferenceAtx: others2Atx.ID(),
				Signature:    otherSig2.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
		}
		atx2.Sign(sig)
		atxHandler.expectAtxV2(atx2)

		verifier := wire.NewMockMalfeasanceValidator(atxHandler.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHandler.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		atxHandler.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			sig.NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofDoubleMarry{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofDoubleMarry)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, sig.NodeID(), nId)
			return nil
		})
		err = atxHandler.processATX(context.Background(), "", atx2, time.Now())
		require.NoError(t, err)

		// The equivocation set of sig and otherSig were merged
		id, err := marriage.FindIDByNodeID(atxHandler.cdb, sig.NodeID())
		require.NoError(t, err)
		equiv, err := marriage.NodeIDsByID(atxHandler.cdb, id)
		require.NoError(t, err)
		require.ElementsMatch(t, []types.NodeID{sig.NodeID(), otherSig.NodeID(), otherSig2.NodeID()}, equiv)
	})
	t.Run("marring into existing malicious equivocation set sets identity as malicious", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		atx, _ := marryIDs(t, atxHandler, []*signing.EdSigner{sig, otherSig}, golden)

		// sig becomes malicious in some way and with it otherSig
		id, err := marriage.FindIDByNodeID(atxHandler.cdb, sig.NodeID())
		require.NoError(t, err)
		require.NoError(t, malfeasance.AddProof(atxHandler.cdb, sig.NodeID(), &id, []byte("proof"), 0, time.Now()))
		require.NoError(t, malfeasance.SetMalicious(atxHandler.cdb, otherSig.NodeID(), id, time.Now()))

		// otherSig2 cannot marry sig, trying to extend its set.
		otherSig2, err := signing.NewEdSigner()
		require.NoError(t, err)
		others2Atx := atxHandler.createAndProcessInitial(otherSig2)
		atx2 := newSoloATXv2(t, atx.PublishEpoch+1, atx.ID(), atx.ID())
		atx2.Marriages = []wire.MarriageCertificate{
			{
				Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
			{
				ReferenceAtx: others2Atx.ID(),
				Signature:    otherSig2.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
		}
		atx2.Sign(sig)
		atxHandler.expectAtxV2(atx2)
		err = atxHandler.processATX(context.Background(), "", atx2, time.Now())
		require.NoError(t, err)

		// The equivocation set of sig and otherSig were merged
		id, err = marriage.FindIDByNodeID(atxHandler.cdb, sig.NodeID())
		require.NoError(t, err)
		equiv, err := marriage.NodeIDsByID(atxHandler.cdb, id)
		require.NoError(t, err)
		require.ElementsMatch(t, []types.NodeID{sig.NodeID(), otherSig.NodeID(), otherSig2.NodeID()}, equiv)
	})
	t.Run("signer must marry self", func(t *testing.T) {
		t.Parallel()
		atxHandler := newV2TestHandler(t, golden)

		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		othersAtx := atxHandler.createAndProcessInitial(otherSig)

		atx := newInitialATXv2(t, golden)
		atx.Marriages = []wire.MarriageCertificate{
			{
				ReferenceAtx: othersAtx.ID(),
				Signature:    otherSig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
		}
		atx.Sign(sig)

		atxHandler.mClock.EXPECT().CurrentLayer().AnyTimes()
		err = atxHandler.processATX(context.Background(), "", atx, time.Now())
		require.ErrorContains(t, err, "signer must marry itself")
		require.ErrorIs(t, err, pubsub.ErrValidationReject)
	})
}

func Test_MarryingMalicious(t *testing.T) {
	t.Parallel()
	golden := types.RandomATXID()
	sig, err := signing.NewEdSigner()
	require.NoError(t, err)

	otherSig, err := signing.NewEdSigner()
	require.NoError(t, err)

	tc := func(malicious types.NodeID) func(t *testing.T) {
		return func(t *testing.T) {
			atxHandler := newV2TestHandler(t, golden)
			othersAtx := atxHandler.createAndProcessInitial(otherSig)

			atx := newInitialATXv2(t, golden)
			atx.Marriages = []wire.MarriageCertificate{
				{
					Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
				}, {
					ReferenceAtx: othersAtx.ID(),
					Signature:    otherSig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
				},
			}
			atx.Sign(sig)
			require.NoError(t, malfeasance.AddProof(atxHandler.cdb, malicious, nil, []byte("proof"), 0, time.Now()))

			atxHandler.expectInitialAtxV2(atx)
			err := atxHandler.processATX(context.Background(), "", atx, time.Now())
			require.NoError(t, err)

			id, err := marriage.FindIDByNodeID(atxHandler.cdb, sig.NodeID())
			require.NoError(t, err)
			equiv, err := marriage.NodeIDsByID(atxHandler.cdb, id)
			require.NoError(t, err)
			require.ElementsMatch(t, []types.NodeID{sig.NodeID(), otherSig.NodeID()}, equiv)

			for _, id := range []types.NodeID{sig.NodeID(), otherSig.NodeID()} {
				m, err := malfeasance.IsMalicious(atxHandler.cdb, id)
				require.NoError(t, err)
				require.True(t, m, "expected %s to be malicious, owner %s other %s", id, sig, otherSig)
			}
		}
	}

	t.Run("owner is malicious", tc(sig.NodeID()))

	t.Run("other is malicious", tc(otherSig.NodeID()))
}

func Test_CalculatingUnits(t *testing.T) {
	t.Parallel()
	t.Run("units on 1 nipost must not overflow", func(t *testing.T) {
		t.Parallel()
		ns := nipostSize{}
		require.NoError(t, ns.addUnits(1))
		require.EqualValues(t, 1, ns.units)
		require.Error(t, ns.addUnits(math.MaxUint32))
	})
	t.Run("total units on all niposts must not overflow", func(t *testing.T) {
		t.Parallel()
		ns := make(nipostSizes, 0)
		ns = append(ns, &nipostSize{units: 11}, &nipostSize{units: math.MaxUint32 - 10})
		_, _, err := ns.sumUp()
		require.Error(t, err)
	})
	t.Run("units = sum of units on every nipost", func(t *testing.T) {
		t.Parallel()
		ns := make(nipostSizes, 0)
		ns = append(ns, &nipostSize{units: 1}, &nipostSize{units: 10})
		u, _, err := ns.sumUp()
		require.NoError(t, err)
		require.EqualValues(t, 1+10, u)
	})
}

func TestContextual_PreviousATX(t *testing.T) {
	t.Run("invalid previous ATX, both v2", func(t *testing.T) {
		golden := types.RandomATXID()
		atxHdlr := newV2TestHandler(t, golden)
		var (
			signers []*signing.EdSigner
			eqSet   []types.NodeID
		)
		for range 3 {
			sig, err := signing.NewEdSigner()
			require.NoError(t, err)
			signers = append(signers, sig)
			eqSet = append(eqSet, sig.NodeID())
		}

		mATX, otherAtxs := marryIDs(t, atxHdlr, signers, golden)

		// signer 1 creates a solo ATX
		soloAtx := newSoloATXv2(t, mATX.PublishEpoch+1, otherAtxs[0].ID(), mATX.ID())
		soloAtx.Sign(signers[1])
		atxHdlr.expectAtxV2(soloAtx)
		err := atxHdlr.processATX(context.Background(), "", soloAtx, time.Now())
		require.NoError(t, err)

		// create a MergedATX for all IDs
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		post := wire.SubPostV2{
			MarriageIndex: 1,
			PrevATXIndex:  1,
			NumUnits:      soloAtx.TotalNumUnits(),
		}
		merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		// Pass a wrong previous ATX for signer 1. It's already been used for soloATX
		// (which should be used for the previous ATX for signer 1).
		merged.PreviousATXs = append(merged.PreviousATXs, otherAtxs[0].ID())
		matxID := mATX.ID()
		merged.MarriageATX = &matxID
		merged.Sign(signers[0])

		atxHdlr.expectMergedAtxV2(merged, eqSet, []uint64{100})

		verifier := wire.NewMockMalfeasanceValidator(atxHdlr.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHdlr.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		atxHdlr.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			signers[1].NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofInvalidPrevAtxV2{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofInvalidPrevAtxV2)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, signers[1].NodeID(), nId)
			return nil
		})

		err = atxHdlr.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)
	})

	t.Run("invalid previous ATX, v1 and v2", func(t *testing.T) {
		golden := types.RandomATXID()
		atxHdlr := newTestHandler(t, golden)

		sig1, err := signing.NewEdSigner()
		require.NoError(t, err)

		// signer 1 creates a solo ATX
		prevATX := newInitialATXv1(t, golden)
		prevATX.Sign(sig1)
		atxHdlr.expectAtxV1(prevATX, prevATX.SmesherID)
		require.NoError(t, atxHdlr.v1.processATX(context.Background(), p2p.NoPeer, prevATX, time.Now()))
		atxv1 := newChainedActivationTxV1(t, prevATX, prevATX.ID())
		atxv1.Sign(sig1)
		atxHdlr.expectAtxV1(atxv1, atxv1.SmesherID)
		require.NoError(t, atxHdlr.v1.processATX(context.Background(), p2p.NoPeer, atxv1, time.Now()))

		soloAtx := newSoloATXv2(t, atxv1.PublishEpoch+1, atxv1.ID(), atxv1.ID())
		soloAtx.Sign(sig1)
		atxHdlr.expectAtxV2(soloAtx)
		err = atxHdlr.v2.processATX(context.Background(), "", soloAtx, time.Now())
		require.NoError(t, err)

		sig2, err := signing.NewEdSigner()
		require.NoError(t, err)
		mATX := newInitialATXv2(t, golden)
		mATX.Marriages = []wire.MarriageCertificate{
			{
				ReferenceAtx: types.EmptyATXID,
				Signature:    sig2.Sign(signing.MARRIAGE, sig2.NodeID().Bytes()),
			},
			{
				ReferenceAtx: soloAtx.ID(),
				Signature:    sig1.Sign(signing.MARRIAGE, sig2.NodeID().Bytes()),
			},
		}
		mATX.PublishEpoch = soloAtx.PublishEpoch
		mATX.Sign(sig2)
		atxHdlr.expectInitialAtxV2(mATX)
		err = atxHdlr.v2.processATX(context.Background(), "", mATX, time.Now())
		require.NoError(t, err)

		// create a MergedATX for all IDs
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		post := wire.SubPostV2{
			MarriageIndex: 1,
			PrevATXIndex:  1,
			NumUnits:      soloAtx.TotalNumUnits(),
		}
		merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)
		merged.PreviousATXs = append(merged.PreviousATXs, prevATX.ID())
		merged.MarriageATX = new(types.ATXID)
		*merged.MarriageATX = mATX.ID()
		merged.Sign(sig2)

		atxHdlr.expectMergedAtxV2(merged, []types.NodeID{sig1.NodeID(), sig2.NodeID()}, []uint64{100})

		verifier := wire.NewMockMalfeasanceValidator(atxHdlr.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHdlr.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		atxHdlr.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			sig1.NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofInvalidPrevAtxV1{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofInvalidPrevAtxV1)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, sig1.NodeID(), nId)
			return nil
		})

		err = atxHdlr.v2.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)
	})

	t.Run("double publish", func(t *testing.T) {
		t.Parallel()
		golden := types.RandomATXID()
		sig, err := signing.NewEdSigner()
		require.NoError(t, err)

		atxHdlr := newV2TestHandler(t, golden)

		// marry
		otherSig, err := signing.NewEdSigner()
		require.NoError(t, err)
		othersAtx := atxHdlr.createAndProcessInitial(otherSig)

		mATX := newInitialATXv2(t, golden)
		mATX.Marriages = []wire.MarriageCertificate{
			{
				Signature: sig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
			{
				ReferenceAtx: othersAtx.ID(),
				Signature:    otherSig.Sign(signing.MARRIAGE, sig.NodeID().Bytes()),
			},
		}
		mATX.Sign(sig)

		atxHdlr.expectInitialAtxV2(mATX)
		err = atxHdlr.processATX(context.Background(), "", mATX, time.Now())
		require.NoError(t, err)

		// publish merged
		merged := newSoloATXv2(t, mATX.PublishEpoch+2, mATX.ID(), mATX.ID())
		post := wire.SubPostV2{
			MarriageIndex: 1,
			NumUnits:      othersAtx.TotalNumUnits(),
			PrevATXIndex:  1,
		}
		merged.NIPosts[0].Posts = append(merged.NIPosts[0].Posts, post)

		mATXID := mATX.ID()
		merged.MarriageATX = &mATXID

		merged.PreviousATXs = []types.ATXID{mATX.ID(), othersAtx.ID()}
		merged.Sign(sig)

		atxHdlr.expectMergedAtxV2(merged, []types.NodeID{sig.NodeID(), otherSig.NodeID()}, []uint64{poetLeaves})
		err = atxHdlr.processATX(context.Background(), "", merged, time.Now())
		require.NoError(t, err)

		// The otherSig tries to publish alone in the same epoch.
		// This is malfeasance as it tries include his PoST twice.
		doubled := newSoloATXv2(t, merged.PublishEpoch, othersAtx.ID(), othersAtx.ID())
		doubled.Sign(otherSig)
		atxHdlr.expectAtxV2(doubled)

		verifier := wire.NewMockMalfeasanceValidator(atxHdlr.ctrl)
		verifier.EXPECT().Signature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(d signing.Domain, nodeID types.NodeID, m []byte, sig types.EdSignature) bool {
				return atxHdlr.edVerifier.Verify(d, nodeID, m, sig)
			}).AnyTimes()

		atxHdlr.mMalPublish.EXPECT().Publish(
			gomock.Any(),
			otherSig.NodeID(),
			gomock.AssignableToTypeOf(&wire.ProofInvalidPrevAtxV2{}),
		).DoAndReturn(func(ctx context.Context, _ types.NodeID, proof wire.Proof) error {
			malProof := proof.(*wire.ProofInvalidPrevAtxV2)
			nId, err := malProof.Valid(ctx, verifier)
			require.NoError(t, err)
			require.Equal(t, otherSig.NodeID(), nId)
			return nil
		})

		err = atxHdlr.processATX(context.Background(), "", doubled, time.Now())
		require.NoError(t, err)
	})
}

func Test_CalculatingWeight(t *testing.T) {
	t.Parallel()
	t.Run("total weight must not overflow uint64", func(t *testing.T) {
		t.Parallel()
		ns := make(nipostSizes, 0)
		ns = append(ns, &nipostSize{units: 1, ticks: 100}, &nipostSize{units: 10, ticks: math.MaxUint64})
		_, _, err := ns.sumUp()
		require.Error(t, err)
	})
	t.Run("weight = sum of weight on every nipost", func(t *testing.T) {
		t.Parallel()
		ns := make(nipostSizes, 0)
		ns = append(ns, &nipostSize{units: 1, ticks: 100}, &nipostSize{units: 10, ticks: 1000})
		_, w, err := ns.sumUp()
		require.NoError(t, err)
		require.EqualValues(t, 1*100+10*1000, w)
	})
}

func Test_CalculatingTicks(t *testing.T) {
	ns := make(nipostSizes, 0)
	ns = append(ns, &nipostSize{units: 1, ticks: 100}, &nipostSize{units: 10, ticks: 1000})
	require.EqualValues(t, 100, ns.minTicks())
}

func newInitialATXv2(tb testing.TB, golden types.ATXID) *wire.ActivationTxV2 {
	tb.Helper()
	atx := &wire.ActivationTxV2{
		PositioningATX: golden,
		Initial:        &wire.InitialAtxPartsV2{CommitmentATX: golden},
		NIPosts: []wire.NIPostV2{
			{
				Membership: wire.MerkleProofV2{
					Nodes: make([]types.Hash32, 32),
				},
				Challenge: types.RandomHash(),
				Posts: []wire.SubPostV2{
					{
						NumUnits: 4,
					},
				},
			},
		},
		Coinbase: types.GenerateAddress([]byte("aaaa")),
		VRFNonce: uint64(999),
	}

	return atx
}

func newSoloATXv2(tb testing.TB, publish types.EpochID, prev, pos types.ATXID) *wire.ActivationTxV2 {
	tb.Helper()

	atx := &wire.ActivationTxV2{
		PublishEpoch:   publish,
		PreviousATXs:   []types.ATXID{prev},
		PositioningATX: pos,
		NIPosts: []wire.NIPostV2{
			{
				Challenge: types.RandomHash(),
				Posts: []wire.SubPostV2{
					{
						NumUnits: 4,
					},
				},
			},
		},
		Coinbase: types.GenerateAddress([]byte("aaaa")),
		VRFNonce: uint64(999),
	}

	return atx
}
