package peersync

import (
	"context"
	"fmt"
	"testing"
	"time"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/spacemeshos/go-scale/tester"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/timesync/peersync/mocks"
)

type adjustedTime time.Time

func (t adjustedTime) Now() time.Time {
	return time.Time(t)
}

type delayedTime time.Duration

func (t delayedTime) Now() time.Time {
	return time.Now().Add(time.Duration(t))
}

func TestSyncGetOffset(t *testing.T) {
	var (
		start           = time.Time{}
		roundStartTime  = start.Add(10 * time.Second)
		peerResponse    = start.Add(30 * time.Second)
		responseReceive = start.Add(40 * time.Second)
	)

	t.Run("Success", func(t *testing.T) {
		mesh, err := mocknet.FullMeshConnected(4)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		tm := mocks.NewMockTime(ctrl)
		peers := []p2p.Peer{}
		tm.EXPECT().Now().Return(roundStartTime)
		tm.EXPECT().Now().Return(responseReceive).AnyTimes()
		for _, h := range mesh.Hosts()[1:] {
			peers = append(peers, h.ID())
			require.NotNil(t, New(h, nil, WithTime(adjustedTime(peerResponse))))
		}
		sync := New(mesh.Hosts()[0], nil, WithTime(tm))
		offset, err := sync.GetOffset(context.TODO(), 0, peers)
		require.NoError(t, err)
		require.Equal(t, 5*time.Second, offset)
	})

	t.Run("Failure", func(t *testing.T) {
		mesh, err := mocknet.FullMeshConnected(4)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		tm := mocks.NewMockTime(ctrl)
		peers := []p2p.Peer{}
		tm.EXPECT().Now().Return(roundStartTime)
		tm.EXPECT().Now().Return(responseReceive).AnyTimes()
		for _, h := range mesh.Hosts()[1:] {
			peers = append(peers, h.ID())
		}

		sync := New(mesh.Hosts()[0], nil, WithTime(tm))
		offset, err := sync.GetOffset(context.TODO(), 0, peers)
		require.ErrorIs(t, err, errTimesyncFailed)
		require.Empty(t, offset)
	})
}

func TestSyncTerminateOnError(t *testing.T) {
	// NOTE(dshulyak) -coverprofile doesn't seem to track code that is executed no in the main goroutine

	config := DefaultConfig()
	config.MaxClockOffset = 1 * time.Second
	config.MaxOffsetErrors = 1
	config.RoundInterval = time.Duration(0)

	var (
		start           = time.Time{}
		roundStartTime  = start.Add(10 * time.Second)
		peerResponse    = start.Add(30 * time.Second)
		responseReceive = start.Add(30 * time.Second)
	)

	mesh, err := mocknet.FullMeshConnected(4)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	getter := mocks.NewMockgetPeers(ctrl)
	tm := mocks.NewMockTime(ctrl)

	sync := New(mesh.Hosts()[0], getter,
		WithTime(tm),
		WithConfig(config),
	)
	tm.EXPECT().Now().Return(roundStartTime)
	tm.EXPECT().Now().Return(responseReceive).AnyTimes()

	peers := []p2p.Peer{}
	for _, h := range mesh.Hosts()[1:] {
		peers = append(peers, h.ID())
		require.NotNil(t, New(h, nil, WithTime(adjustedTime(peerResponse))))
	}
	getter.EXPECT().GetPeers().Return(peers)

	sync.Start()
	t.Cleanup(sync.Stop)
	errors := make(chan error, 1)
	go func() {
		errors <- sync.Wait()
	}()
	select {
	case err := <-errors:
		require.ErrorIs(t, err, errPeersNotSynced)
	case <-time.After(100 * time.Millisecond):
		require.FailNow(t, "timed out waiting for sync to fail")
	}
}

func TestSyncSimulateMultiple(t *testing.T) {
	config := DefaultConfig()
	config.MaxClockOffset = 1 * time.Second
	config.MaxOffsetErrors = 2
	config.RoundInterval = 0

	delays := []time.Duration{0, 1200 * time.Millisecond, 1900 * time.Millisecond, 10 * time.Second}
	instances := []*Sync{}
	errors := []error{errPeersNotSynced, nil, nil, errPeersNotSynced}
	mesh, err := mocknet.FullMeshLinked(len(delays))
	require.NoError(t, err)
	hosts := []*p2p.Host{}
	for _, h := range mesh.Hosts() {
		fh, err := p2p.Upgrade(h)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, fh.Stop()) })
		hosts = append(hosts, fh)
	}
	require.NoError(t, mesh.ConnectAllButSelf())

	// First create all instances so they register in the protocol
	// and then start them.
	for i, delay := range delays {
		sync := New(hosts[i], hosts[i],
			WithConfig(config),
			WithTime(delayedTime(delay)),
			WithLog(zaptest.NewLogger(t).Named(fmt.Sprintf("%d-%s", i, hosts[i].ID()))),
		)
		instances = append(instances, sync)
	}
	for _, sync := range instances {
		sync.Start()
		t.Cleanup(sync.Stop)
	}
	for i, inst := range instances {
		if errors[i] == nil {
			continue
		}
		wait := make(chan error, 1)
		go func() {
			wait <- inst.Wait()
		}()
		select {
		case err := <-wait:
			require.ErrorIs(t, err, errors[i])
		case <-time.After(1000 * time.Millisecond):
			require.FailNowf(t, "timed out waiting for an error", "node %d", i)
		}
	}
}

func FuzzRequestConsistency(f *testing.F) {
	tester.FuzzConsistency[Request](f)
}

func FuzzRequestSafety(f *testing.F) {
	tester.FuzzSafety[Request](f)
}

func FuzzResponseConsistency(f *testing.F) {
	tester.FuzzConsistency[Response](f)
}

func FuzzResponseSafety(f *testing.F) {
	tester.FuzzSafety[Response](f)
}
