package transactions_test

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/genvm/sdk"
	"github.com/spacemeshos/go-spacemesh/genvm/sdk/wallet"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/statesql"
	"github.com/spacemeshos/go-spacemesh/sql/transactions"
)

func createTX(
	tb testing.TB,
	principal *signing.EdSigner,
	dest types.Address,
	nonce, amount, fee uint64,
) *types.Transaction {
	tb.Helper()

	var raw []byte
	if nonce == 0 {
		raw = wallet.SelfSpawn(principal.PrivateKey(), 0, sdk.WithGasPrice(fee))
	} else {
		raw = wallet.Spend(principal.PrivateKey(), dest, amount,
			nonce, sdk.WithGasPrice(fee))
	}

	parsed := types.Transaction{
		RawTx:    types.NewRawTx(raw),
		TxHeader: &types.TxHeader{},
	}
	// this is a fake principal for the purposes of testing.
	addr := types.GenerateAddress(principal.PublicKey().Bytes())
	copy(parsed.Principal[:], addr.Bytes())
	parsed.Nonce = nonce
	parsed.GasPrice = fee
	return &parsed
}

func makeMeshTX(
	tx *types.Transaction,
	lid types.LayerID,
	bid types.BlockID,
	received time.Time,
	state types.TXState,
) *types.MeshTransaction {
	return &types.MeshTransaction{
		Transaction: *tx,
		LayerID:     lid,
		BlockID:     bid,
		Received:    received,
		State:       state,
	}
}

func checkMeshTXEqual(tb testing.TB, expected, got types.MeshTransaction) {
	tb.Helper()
	require.EqualValues(tb, expected.Received.UnixNano(), got.Received.UnixNano())
	got.Received = time.Time{}
	expected.Received = time.Time{}
	require.Equal(tb, expected, got)
}

func TestAddGetHas(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	signer1, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	signer2, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	txs := []*types.Transaction{
		createTX(t, signer1, types.Address{1}, 1, 191, 1),
		createTX(t, signer2, types.Address{2}, 1, 191, 1),
		createTX(t, signer1, types.Address{3}, 1, 191, 1),
	}

	received := time.Now()
	for _, tx := range txs {
		require.NoError(t, transactions.Add(db, tx, received))
	}

	for _, tx := range txs {
		got, err := transactions.Get(db, tx.ID)
		require.NoError(t, err)
		expected := makeMeshTX(tx, 0, types.EmptyBlockID, received, types.MEMPOOL)
		checkMeshTXEqual(t, *expected, *got)

		has, err := transactions.Has(db, tx.ID)
		require.NoError(t, err)
		require.True(t, has)
	}

	tid := types.RandomTransactionID()
	_, err = transactions.Get(db, tid)
	require.ErrorIs(t, err, sql.ErrNotFound)

	has, err := transactions.Has(db, tid)
	require.NoError(t, err)
	require.False(t, has)
}

func TestAddUpdatesHeader(t *testing.T) {
	db := statesql.InMemoryTest(t)
	txs := []*types.Transaction{
		{
			RawTx:    types.NewRawTx([]byte{1, 2, 3}),
			TxHeader: &types.TxHeader{Principal: types.Address{1}},
		},
		{
			RawTx: types.NewRawTx([]byte{4, 5, 6}),
		},
	}
	require.NoError(t, transactions.Add(db, txs[1], time.Time{}))

	require.NoError(t, transactions.Add(db, &types.Transaction{RawTx: txs[0].RawTx}, time.Time{}))
	tx, err := transactions.Get(db, txs[0].ID)
	require.NoError(t, err)
	require.Nil(t, tx.TxHeader)

	require.NoError(t, transactions.Add(db, txs[0], time.Time{}))
	tx, err = transactions.Get(db, txs[0].ID)
	require.NoError(t, err)
	require.NotNil(t, tx.TxHeader)

	require.NoError(t, transactions.Add(db, &types.Transaction{RawTx: txs[0].RawTx}, time.Time{}))
	tx, err = transactions.Get(db, txs[0].ID)
	require.NoError(t, err)
	require.NotNil(t, tx.TxHeader)

	tx, err = transactions.Get(db, txs[1].ID)
	require.NoError(t, err)
	require.Nil(t, tx.TxHeader)
}

func TestAddToProposal(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1)
	require.NoError(t, transactions.Add(db, tx, time.Now()))

	lid := types.LayerID(10)
	pid := types.ProposalID{1, 1}
	require.NoError(t, transactions.AddToProposal(db, tx.ID, lid, pid))
	// do it again
	require.NoError(t, transactions.AddToProposal(db, tx.ID, lid, pid))

	has, err := transactions.HasProposalTX(db, pid, tx.ID)
	require.NoError(t, err)
	require.True(t, has)

	has, err = transactions.HasProposalTX(db, types.ProposalID{2, 2}, tx.ID)
	require.NoError(t, err)
	require.False(t, has)
}

func TestDeleteProposalTxs(t *testing.T) {
	db := statesql.InMemoryTest(t)
	proposals := map[types.LayerID][]types.ProposalID{
		types.LayerID(10): {{1, 1}, {1, 2}},
		types.LayerID(11): {{2, 1}, {2, 2}},
	}
	tids := []types.TransactionID{{1, 2}, {2, 3}}
	for lid, pids := range proposals {
		for _, tid := range tids {
			for _, pid := range pids {
				require.NoError(t, transactions.AddToProposal(db, tid, lid, pid))
			}
		}
	}
	require.NoError(t, transactions.DeleteProposalTxsBefore(db, types.LayerID(11)))
	for _, pid := range proposals[types.LayerID(10)] {
		for _, tid := range tids {
			has, err := transactions.HasProposalTX(db, pid, tid)
			require.NoError(t, err)
			require.False(t, has)
		}
	}
	for _, pid := range proposals[types.LayerID(11)] {
		for _, tid := range tids {
			has, err := transactions.HasProposalTX(db, pid, tid)
			require.NoError(t, err)
			require.True(t, has)
		}
	}
}

func TestAddToBlock(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1)
	require.NoError(t, transactions.Add(db, tx, time.Now()))

	lid := types.LayerID(10)
	bid := types.BlockID{1, 1}
	require.NoError(t, transactions.AddToBlock(db, tx.ID, lid, bid))
	// do it again
	require.NoError(t, transactions.AddToBlock(db, tx.ID, lid, bid))

	has, err := transactions.HasBlockTX(db, bid, tx.ID)
	require.NoError(t, err)
	require.True(t, has)

	has, err = transactions.HasBlockTX(db, types.BlockID{2, 2}, tx.ID)
	require.NoError(t, err)
	require.False(t, has)
}

func TestApply_AlreadyApplied(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	lid := types.LayerID(10)
	signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1)
	require.NoError(t, transactions.Add(db, tx, time.Now()))

	bid := types.RandomBlockID()
	require.NoError(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.AddResult(dtx, tx.ID, &types.TransactionResult{Layer: lid, Block: bid})
	}))

	// same block applied again
	require.Error(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.AddResult(dtx, tx.ID, &types.TransactionResult{Layer: lid, Block: bid})
	}))

	// different block applied again
	require.Error(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.AddResult(
			dtx,
			tx.ID,
			&types.TransactionResult{Layer: lid.Add(1), Block: types.RandomBlockID()},
		)
	}))
}

func TestUndoLayers_Empty(t *testing.T) {
	db := statesql.InMemoryTest(t)

	require.NoError(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.UndoLayers(dtx, types.LayerID(199))
	}))
}

func TestApplyAndUndoLayers(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	firstLayer := types.LayerID(10)
	numLayers := uint32(5)
	applied := make([]types.TransactionID, 0, numLayers)
	for lid := firstLayer; lid.Before(firstLayer.Add(numLayers)); lid = lid.Add(1) {
		signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
		require.NoError(t, err)
		tx := createTX(t, signer, types.Address{1}, uint64(lid), 191, 2)
		require.NoError(t, transactions.Add(db, tx, time.Now()))
		bid := types.RandomBlockID()

		require.NoError(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
			return transactions.AddResult(dtx, tx.ID, &types.TransactionResult{Layer: lid, Block: bid})
		}))
		applied = append(applied, tx.ID)
	}

	for _, tid := range applied {
		mtx, err := transactions.Get(db, tid)
		require.NoError(t, err)
		require.Equal(t, types.APPLIED, mtx.State)
	}
	// revert to firstLayer
	require.NoError(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.UndoLayers(dtx, firstLayer.Add(1))
	}))

	for i, tid := range applied {
		mtx, err := transactions.Get(db, tid)
		require.NoError(t, err)
		if i == 0 {
			require.Equal(t, types.APPLIED, mtx.State)
		} else {
			require.Equal(t, types.MEMPOOL, mtx.State)
		}
	}
}

func TestGetBlob(t *testing.T) {
	db := statesql.InMemoryTest(t)
	ctx := context.Background()

	rng := rand.New(rand.NewSource(1001))
	numTXs := 5
	txs := make([]*types.Transaction, 0, numTXs)
	for i := 0; i < numTXs; i++ {
		signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
		require.NoError(t, err)
		tx := createTX(t, signer, types.Address{1}, 1, 191, 1)
		require.NoError(t, transactions.Add(db, tx, time.Now()))
		txs = append(txs, tx)
	}

	noSuchID := types.RandomTransactionID()
	require.ErrorIs(t, transactions.LoadBlob(ctx, db, noSuchID[:], &sql.Blob{}), sql.ErrNotFound)

	ids := [][]byte{noSuchID[:]}
	expSizes := []int{-1}
	for _, tx := range txs {
		var blob sql.Blob
		require.NoError(t, transactions.LoadBlob(ctx, db, tx.ID[:], &blob))
		require.Equal(t, tx.Raw, blob.Bytes)
		ids = append(ids, tx.ID[:])
		expSizes = append(expSizes, len(tx.Raw))
	}

	blobSizes, err := transactions.GetBlobSizes(db, ids)
	require.NoError(t, err)
	require.Equal(t, expSizes, blobSizes)
}

func TestGetByAddress(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	signer1, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	signer2, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	signer2Address := types.GenerateAddress(signer2.PublicKey().Bytes())
	lid := types.LayerID(10)
	txs := []*types.Transaction{
		createTX(t, signer1, types.Address{1}, 1, 191, 1),
		createTX(t, signer2, types.Address{2}, 1, 191, 1),
		createTX(t, signer1, signer2Address, 1, 191, 1),
	}
	received := time.Now()
	require.NoError(t, db.WithTxImmediate(context.Background(), func(dbtx sql.Transaction) error {
		for _, tx := range txs {
			require.NoError(t, transactions.Add(dbtx, tx, received))
			require.NoError(t, transactions.AddResult(dbtx, tx.ID, &types.TransactionResult{Layer: lid}))
		}
		return nil
	}))

	// should be nothing before lid
	got, err := transactions.GetByAddress(db, types.LayerID(1), lid.Sub(1), signer2Address)
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = transactions.GetByAddress(db, 0, lid, signer2Address)
	require.NoError(t, err)
	require.Len(t, got, 1)
	expected1 := makeMeshTX(txs[1], lid, types.EmptyBlockID, received, types.APPLIED)
	checkMeshTXEqual(t, *expected1, *got[0])
}

func TestGetAcctPendingFromNonce(t *testing.T) {
	db := statesql.InMemoryTest(t)

	rng := rand.New(rand.NewSource(1001))
	signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	numTXs := 13
	// use math.MaxInt64+1 to validate nonce sqlite comparison in GetAcctPendingFromNonce
	nonce := uint64(math.MaxInt64 + 1)
	received := time.Now()
	for i := 0; i < numTXs; i++ {
		tx := createTX(t, signer, types.Address{1}, nonce+uint64(i), 191, 1)
		require.NoError(t, transactions.Add(db, tx, received.Add(time.Duration(i))))
		if i > 0 {
			tx = createTX(t, signer, types.Address{1}, nonce-uint64(i), 191, 1)
			require.NoError(t, transactions.Add(db, tx, received.Add(time.Duration(i))))
		}
	}

	// create tx for different accounts
	for i := 0; i < numTXs; i++ {
		signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
		require.NoError(t, err)
		tx := createTX(t, signer, types.Address{1}, 1, 191, 1)
		require.NoError(t, transactions.Add(db, tx, received))
	}

	principal := types.GenerateAddress(signer.PublicKey().Bytes())
	for i := 0; i < numTXs; i++ {
		got, err := transactions.GetAcctPendingFromNonce(db, principal, nonce+uint64(i))
		require.NoError(t, err)
		require.Len(t, got, numTXs-i)
	}
}

func TestAppliedLayer(t *testing.T) {
	db := statesql.InMemoryTest(t)
	rng := rand.New(rand.NewSource(1001))
	signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
	require.NoError(t, err)
	txs := []*types.Transaction{
		createTX(t, signer, types.Address{1}, 1, 191, 1),
		createTX(t, signer, types.Address{1}, 2, 191, 1),
	}
	lid := types.LayerID(10)

	for _, tx := range txs {
		require.NoError(t, transactions.Add(db, tx, time.Now()))
	}
	require.NoError(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.AddResult(dtx, txs[0].ID, &types.TransactionResult{Layer: lid, Block: types.BlockID{1, 1}})
	}))

	applied, err := transactions.GetAppliedLayer(db, txs[0].ID)
	require.NoError(t, err)
	require.Equal(t, lid, applied)

	_, err = transactions.GetAppliedLayer(db, txs[1].ID)
	require.ErrorIs(t, err, sql.ErrNotFound)

	require.NoError(t, db.WithTxImmediate(context.Background(), func(dtx sql.Transaction) error {
		return transactions.UndoLayers(dtx, lid)
	}))
	_, err = transactions.GetAppliedLayer(db, txs[0].ID)
	require.ErrorIs(t, err, sql.ErrNotFound)
}

func TestAddressesWithPendingTransactions(t *testing.T) {
	principals := []types.Address{
		{1},
		{2},
		{3},
	}
	txs := []types.Transaction{
		{
			RawTx:    types.RawTx{ID: types.TransactionID{1}},
			TxHeader: &types.TxHeader{Principal: principals[0], Nonce: 0},
		},
		{
			RawTx:    types.RawTx{ID: types.TransactionID{2}},
			TxHeader: &types.TxHeader{Principal: principals[0], Nonce: 1},
		},
		{
			RawTx:    types.RawTx{ID: types.TransactionID{3}},
			TxHeader: &types.TxHeader{Principal: principals[1], Nonce: 0},
		},
	}
	db := statesql.InMemoryTest(t)
	for _, tx := range txs {
		require.NoError(t, transactions.Add(db, &tx, time.Time{}))
	}
	rst, err := transactions.AddressesWithPendingTransactions(db)
	require.NoError(t, err)
	require.Equal(t, []types.AddressNonce{
		{Address: principals[0], Nonce: txs[0].Nonce},
		{Address: principals[1], Nonce: txs[2].Nonce},
	}, rst)
	require.NoError(t, db.WithTxImmediate(context.Background(), func(dbtx sql.Transaction) error {
		return transactions.AddResult(dbtx, txs[0].ID, &types.TransactionResult{Message: "hey"})
	}))
	rst, err = transactions.AddressesWithPendingTransactions(db)
	require.NoError(t, err)
	require.Equal(t, []types.AddressNonce{
		{Address: principals[0], Nonce: txs[1].Nonce},
		{Address: principals[1], Nonce: txs[2].Nonce},
	}, rst)
	require.NoError(t, db.WithTxImmediate(context.Background(), func(dbtx sql.Transaction) error {
		return transactions.AddResult(dbtx, txs[2].ID, &types.TransactionResult{Message: "hey"})
	}))
	rst, err = transactions.AddressesWithPendingTransactions(db)
	require.NoError(t, err)
	require.Equal(t, []types.AddressNonce{
		{Address: principals[0], Nonce: txs[1].Nonce},
	}, rst)
	more := []types.Transaction{
		{
			RawTx:    types.RawTx{ID: types.TransactionID{4}},
			TxHeader: &types.TxHeader{Principal: principals[2], Nonce: 0},
		},
		{
			RawTx:    types.RawTx{ID: types.TransactionID{5}},
			TxHeader: &types.TxHeader{Principal: principals[2], Nonce: 1},
		},
		{
			RawTx:    types.RawTx{ID: types.TransactionID{6}},
			TxHeader: &types.TxHeader{Principal: principals[1], Nonce: 1},
		},
	}
	for _, tx := range more {
		require.NoError(t, transactions.Add(db, &tx, time.Time{}))
	}
	rst, err = transactions.AddressesWithPendingTransactions(db)
	require.NoError(t, err)
	require.Equal(t, []types.AddressNonce{
		{Address: principals[0], Nonce: txs[1].Nonce},
		{Address: principals[1], Nonce: more[2].Nonce},
		{Address: principals[2], Nonce: more[0].Nonce},
	}, rst)
}

func TestTransactionInProposal(t *testing.T) {
	tid := types.TransactionID{1}
	lids := []types.LayerID{
		types.LayerID(1),
		types.LayerID(2),
		types.LayerID(3),
	}
	pids := []types.ProposalID{
		{1},
		{2},
		{3},
	}
	db := statesql.InMemoryTest(t)
	for i := range lids {
		require.NoError(t, transactions.AddToProposal(db, tid, lids[i], pids[i]))
	}
	lid, err := transactions.TransactionInProposal(db, tid, 0)
	require.NoError(t, err)
	require.Equal(t, lids[0], lid)
	lid, err = transactions.TransactionInProposal(db, tid, lids[1])
	require.NoError(t, err)
	require.Equal(t, lids[2], lid)
	_, err = transactions.TransactionInProposal(db, tid, lids[2])
	require.ErrorIs(t, err, sql.ErrNotFound)
}

func TestTransactionInBlock(t *testing.T) {
	tid := types.TransactionID{1}
	lids := []types.LayerID{
		types.LayerID(1),
		types.LayerID(2),
		types.LayerID(3),
	}
	bids := []types.BlockID{
		{1},
		{2},
		{3},
	}
	db := statesql.InMemoryTest(t)
	for i := range lids {
		require.NoError(t, transactions.AddToBlock(db, tid, lids[i], bids[i]))
	}
	bid, lid, err := transactions.TransactionInBlock(db, tid, 0)
	require.NoError(t, err)
	require.Equal(t, lids[0], lid)
	require.Equal(t, bids[0], bid)
	bid, lid, err = transactions.TransactionInBlock(db, tid, lids[1])
	require.NoError(t, err)
	require.Equal(t, lids[2], lid)
	require.Equal(t, bids[2], bid)
	_, _, err = transactions.TransactionInBlock(db, tid, lids[2])
	require.ErrorIs(t, err, sql.ErrNotFound)
}

func TestTransactionEvictMempool(t *testing.T) {
	principals := []types.Address{
		{1},
		{2},
		{3},
	}
	txs := []types.Transaction{
		{
			RawTx:    types.RawTx{ID: types.TransactionID{1}},
			TxHeader: &types.TxHeader{Principal: principals[0], Nonce: 0},
		},
		{
			RawTx:    types.RawTx{ID: types.TransactionID{2}},
			TxHeader: &types.TxHeader{Principal: principals[0], Nonce: 1},
		},
		{
			RawTx:    types.RawTx{ID: types.TransactionID{3}},
			TxHeader: &types.TxHeader{Principal: principals[1], Nonce: 0},
		},
	}
	db := statesql.InMemoryTest(t)
	for _, tx := range txs {
		require.NoError(t, transactions.Add(db, &tx, time.Time{}))
	}
	err := transactions.SetEvicted(db, types.TransactionID{1})
	require.NoError(t, err)

	err = transactions.Delete(db, types.TransactionID{1})
	require.NoError(t, err)

	pending, err := transactions.GetAcctPendingFromNonce(db, principals[0], 1)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, pending[0].ID, txs[1].ID)

	pending, err = transactions.GetAcctPendingFromNonce(db, principals[1], 0)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, pending[0].ID, txs[2].ID)

	has, err := transactions.Has(db, txs[0].ID)
	require.False(t, has)
	require.NoError(t, err)

	has, err = transactions.HasEvicted(db, txs[0].ID)
	require.True(t, has)
	require.NoError(t, err)
}

func TestPruneEvicted(t *testing.T) {
	txId := types.TransactionID{1}
	db := statesql.InMemoryTest(t)
	db.Exec(`insert into evicted_mempool (id, time) values (?1,?2);`,
		func(stmt *sql.Statement) {
			stmt.BindBytes(1, txId.Bytes())
			stmt.BindInt64(2, time.Now().Add(-13*time.Hour).UnixNano())
		}, nil)

	has, err := transactions.HasEvicted(db, txId)
	require.True(t, has)
	require.NoError(t, err)

	err = transactions.PruneEvicted(db, time.Now().Add(-12*time.Hour))
	require.NoError(t, err)

	has, err = transactions.HasEvicted(db, txId)
	require.False(t, has)
	require.NoError(t, err)
}
