package multipeer_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/fetch/peers"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/sync2/multipeer"
	"github.com/spacemeshos/go-spacemesh/sync2/rangesync"
)

const (
	numSyncs     = 3
	numSyncPeers = 6
)

// FIXME: BlockUntilContext is not included in FakeClock interface.
// This will be fixed in a post-0.4.0 clockwork release, but with a breaking change that
// makes FakeClock a struct instead of an interface.
// See: https://github.com/jonboulle/clockwork/pull/71
type fakeClock interface {
	clockwork.FakeClock
	BlockUntilContext(ctx context.Context, n int) error
}

type peerList struct {
	sync.Mutex
	peers []p2p.Peer
}

func (pl *peerList) add(p p2p.Peer) bool {
	pl.Lock()
	defer pl.Unlock()
	if slices.Contains(pl.peers, p) {
		return false
	}
	pl.peers = append(pl.peers, p)
	return true
}

func (pl *peerList) get() []p2p.Peer {
	pl.Lock()
	defer pl.Unlock()
	return slices.Clone(pl.peers)
}

type multiPeerSyncTester struct {
	*testing.T
	ctrl       *gomock.Controller
	syncBase   *MockSyncBase
	syncRunner *MocksyncRunner
	peers      *peers.Peers
	clock      fakeClock
	reconciler *multipeer.MultiPeerReconciler
	cancel     context.CancelFunc
	eg         errgroup.Group
	kickCh     chan struct{}
	// EXPECT() calls should not be done concurrently
	// https://github.com/golang/mock/issues/533#issuecomment-821537840
	mtx sync.Mutex
}

func newMultiPeerSyncTester(t *testing.T, addPeers int) *multiPeerSyncTester {
	ctrl := gomock.NewController(t)
	mt := &multiPeerSyncTester{
		T:          t,
		ctrl:       ctrl,
		syncBase:   NewMockSyncBase(ctrl),
		syncRunner: NewMocksyncRunner(ctrl),
		peers:      peers.New(),
		clock:      clockwork.NewFakeClock().(fakeClock),
		kickCh:     make(chan struct{}, 1),
	}
	cfg := multipeer.DefaultConfig()
	cfg.SyncInterval = 40 * time.Second
	cfg.SyncIntervalSpread = 0.1
	cfg.SyncPeerCount = numSyncPeers
	cfg.RetryInterval = 5 * time.Second
	cfg.MinSplitSyncPeers = 2
	cfg.MinSplitSyncCount = 90
	cfg.MaxFullDiff = 20
	cfg.MinCompleteFraction = 0.9
	cfg.NoPeersRecheckInterval = 10 * time.Second
	mt.reconciler = multipeer.NewMultiPeerReconcilerInternal(
		zaptest.NewLogger(t), cfg,
		mt.syncBase, mt.peers, 32, 24,
		mt.syncRunner, mt.clock)
	mt.addPeers(addPeers)
	return mt
}

func (mt *multiPeerSyncTester) addPeers(n int) []p2p.Peer {
	r := make([]p2p.Peer, n)
	for i := 0; i < n; i++ {
		p := p2p.Peer(fmt.Sprintf("peer%d", i+1))
		mt.peers.Add(p, func() []protocol.ID { return []protocol.ID{multipeer.Protocol} })
		r[i] = p
	}
	return r
}

func (mt *multiPeerSyncTester) start() context.Context {
	var ctx context.Context
	ctx, mt.cancel = context.WithTimeout(context.Background(), 10*time.Second)
	mt.eg.Go(func() error { return mt.reconciler.Run(ctx, mt.kickCh) })
	mt.Cleanup(func() {
		mt.cancel()
		if err := mt.eg.Wait(); err != nil {
			require.ErrorIs(mt, err, context.Canceled)
		}
	})
	return ctx
}

func (mt *multiPeerSyncTester) kick() {
	mt.kickCh <- struct{}{}
}

func (mt *multiPeerSyncTester) expectProbe(times int, pr rangesync.ProbeResult) *peerList {
	var pl peerList
	mt.syncBase.EXPECT().Probe(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p p2p.Peer) (rangesync.ProbeResult, error) {
			require.True(mt, pl.add(p), "peer shouldn't be probed twice")
			require.True(mt, mt.peers.Contains(p))
			return pr, nil
		}).Times(times)
	return &pl
}

func (mt *multiPeerSyncTester) expectSingleProbe(
	peer p2p.Peer,
	pr rangesync.ProbeResult,
) {
	mt.syncBase.EXPECT().Probe(gomock.Any(), peer).DoAndReturn(
		func(_ context.Context, p p2p.Peer) (rangesync.ProbeResult, error) {
			return pr, nil
		})
}

func (mt *multiPeerSyncTester) expectFullSync(pl *peerList, times, numFails int) {
	mt.syncRunner.EXPECT().FullSync(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, peers []p2p.Peer) error {
			require.ElementsMatch(mt, pl.get(), peers)
			// delegate to the real fullsync
			return mt.reconciler.FullSync(ctx, peers)
		})
	mt.syncBase.EXPECT().
		Sync(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, p p2p.Peer, x, y rangesync.KeyBytes) error {
			mt.mtx.Lock()
			defer mt.mtx.Unlock()
			require.Contains(mt, pl.get(), p)
			if numFails != 0 {
				numFails--
				return errors.New("sync failed")
			}
			return nil
		}).Times(times)
}

// satisfy waits until all the expected mocked calls are made.
func (mt *multiPeerSyncTester) satisfy() {
	require.Eventually(mt, func() bool {
		mt.mtx.Lock()
		defer mt.mtx.Unlock()
		return mt.ctrl.Satisfied()
	}, time.Second, time.Millisecond)
}

func TestMultiPeerSync(t *testing.T) {
	t.Run("split sync", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 0)
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		// Advance by sync interval (incl. spread). No peers yet
		mt.clock.Advance(time.Minute)
		mt.clock.BlockUntilContext(ctx, 1)
		// It is safe to do EXPECT() calls while the MultiPeerReconciler is blocked
		mt.addPeers(10)
		// Advance by peer wait time. After that, 6 peers will be selected
		// randomly and probed
		mt.syncBase.EXPECT().Count().Return(50, nil).AnyTimes()
		for i := 0; i < numSyncs; i++ {
			plSplit := mt.expectProbe(numSyncPeers, rangesync.ProbeResult{
				InSync: false,
				Count:  100,
				Sim:    0.5, // too low for full sync
			})
			mt.syncRunner.EXPECT().SplitSync(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, peers []p2p.Peer) error {
					require.ElementsMatch(t, plSplit.get(), peers)
					return nil
				})
			mt.clock.BlockUntilContext(ctx, 1)
			mt.expectProbe(numSyncPeers, rangesync.ProbeResult{
				InSync: true,
				Count:  100,
				Sim:    1, // after sync
			})
			// no full sync here as the node is already in sync according to the probe above
			if i > 0 {
				mt.clock.Advance(time.Minute)
			} else if i < numSyncs-1 {
				mt.clock.Advance(10 * time.Second)
			}
			mt.satisfy()
		}
	})

	t.Run("full sync", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		require.False(t, mt.reconciler.Synced())
		expect := func() {
			pl := mt.expectProbe(numSyncPeers, rangesync.ProbeResult{
				InSync: false,
				Count:  100,
				Sim:    0.99, // high enough for full sync
			})
			mt.expectFullSync(pl, numSyncPeers, 0)
		}
		expect()
		// first probe and first full sync happen immediately
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 0; i < numSyncs; i++ {
			expect()
			mt.clock.Advance(time.Minute)
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("some in sync", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		require.False(t, mt.reconciler.Synced())
		expect := func() {
			pl := mt.expectProbe(numSyncPeers-1, rangesync.ProbeResult{
				InSync: false,
				Count:  100,
				Sim:    0.99,
			})
			mt.expectFullSync(pl, numSyncPeers-1, 0)
			mt.expectProbe(1, rangesync.ProbeResult{
				InSync: true,
				Count:  100,
				Sim:    1,
			})
		}
		expect()
		// first probe happens immediately
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 0; i < numSyncs; i++ {
			expect()
			mt.clock.Advance(time.Minute)
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("all in sync", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		require.False(t, mt.reconciler.Synced())
		expect := func() {
			mt.expectProbe(numSyncPeers, rangesync.ProbeResult{
				InSync: true,
				Count:  100,
				Sim:    1,
			})
		}
		expect()
		// first probe happens immediately
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 0; i < numSyncs; i++ {
			expect()
			mt.clock.Advance(time.Minute)
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("sync after kick", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		require.False(t, mt.reconciler.Synced())
		expect := func() {
			pl := mt.expectProbe(numSyncPeers, rangesync.ProbeResult{
				InSync: false,
				Count:  100,
				Sim:    0.99, // high enough for full sync
			})
			mt.expectFullSync(pl, numSyncPeers, 0)
		}
		expect()
		// first full sync happens immediately
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 0; i < numSyncs; i++ {
			expect()
			mt.kick()
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("full sync, peers with low count ignored", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 0)
		addedPeers := mt.addPeers(numSyncPeers)
		mt.syncBase.EXPECT().Count().Return(1000, nil).AnyTimes()
		require.False(t, mt.reconciler.Synced())
		expect := func() {
			var pl peerList
			for _, p := range addedPeers[:5] {
				mt.expectSingleProbe(p, rangesync.ProbeResult{
					InSync: false,
					Count:  1000,
					Sim:    0.99, // high enough for full sync
				})
				pl.add(p)
			}
			mt.expectSingleProbe(addedPeers[5], rangesync.ProbeResult{
				InSync: false,
				Count:  800, // count too low, this peer should be ignored
				Sim:    0.9,
			})
			mt.expectFullSync(&pl, 5, 0)
		}
		expect()
		// first full sync happens immediately
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 1; i < numSyncs; i++ {
			expect()
			mt.clock.Advance(time.Minute)
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("full sync due to low peer count", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 1)
		mt.syncBase.EXPECT().Count().Return(50, nil).AnyTimes()
		expect := func() {
			pl := mt.expectProbe(1, rangesync.ProbeResult{
				InSync: false,
				Count:  100,
				Sim:    0.5, // too low for full sync, but will have it anyway
			})
			mt.expectFullSync(pl, 1, 0)
		}
		expect()
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 1; i < numSyncs; i++ {
			expect()
			mt.clock.Advance(time.Minute)
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("probe failure", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		mt.syncBase.EXPECT().Probe(gomock.Any(), gomock.Any()).
			Return(rangesync.ProbeResult{}, errors.New("probe failed"))
		pl := mt.expectProbe(5, rangesync.ProbeResult{InSync: false, Count: 100, Sim: 0.99})
		// just 5 peers for which the probe worked will be checked
		mt.expectFullSync(pl, 5, 0)
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
	})

	t.Run("failed peers during full sync", func(t *testing.T) {
		const numFails = 3
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		expect := func() {
			pl := mt.expectProbe(numSyncPeers, rangesync.ProbeResult{InSync: false, Count: 100, Sim: 0.99})
			mt.expectFullSync(pl, numSyncPeers, numFails)
		}
		expect()
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()
		for i := 1; i < numSyncs; i++ {
			expect()
			mt.clock.Advance(time.Minute)
			mt.clock.BlockUntilContext(ctx, 1)
			mt.satisfy()
		}
		require.True(t, mt.reconciler.Synced())
	})

	t.Run("all peers failed during full sync", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()

		pl := mt.expectProbe(numSyncPeers, rangesync.ProbeResult{InSync: false, Count: 100, Sim: 0.99})
		mt.expectFullSync(pl, numSyncPeers, numSyncPeers)

		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		mt.satisfy()

		pl = mt.expectProbe(numSyncPeers, rangesync.ProbeResult{InSync: false, Count: 100, Sim: 0.99})
		mt.expectFullSync(pl, numSyncPeers, 0)
		// Retry should happen after mere 5 seconds as no peers have succeeded, no
		// need to wait full sync interval.
		mt.clock.Advance(5 * time.Second)
		mt.satisfy()

		require.True(t, mt.reconciler.Synced())
	})

	t.Run("cancellation during sync", func(t *testing.T) {
		mt := newMultiPeerSyncTester(t, 10)
		mt.syncBase.EXPECT().Count().Return(100, nil).AnyTimes()
		mt.expectProbe(numSyncPeers, rangesync.ProbeResult{InSync: false, Count: 100, Sim: 0.99})
		mt.syncRunner.EXPECT().FullSync(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, peers []p2p.Peer) error {
				mt.cancel()
				return ctx.Err()
			})
		ctx := mt.start()
		mt.clock.BlockUntilContext(ctx, 1)
		require.ErrorIs(t, mt.eg.Wait(), context.Canceled)
	})
}
