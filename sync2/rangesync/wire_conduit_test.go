package rangesync_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/p2p/server"
	"github.com/spacemeshos/go-spacemesh/sync2/rangesync"
)

type pipeStream struct {
	io.ReadCloser
	io.WriteCloser
}

func (ps *pipeStream) Close() error {
	return errors.Join(ps.ReadCloser.Close(), ps.WriteCloser.Close())
}

type incomingRequest struct {
	initialRequest []byte
	stream         io.ReadWriter
}

type fakeRequester struct {
	t       *testing.T
	id      p2p.Peer
	handler server.StreamHandler
	peers   map[p2p.Peer]*fakeRequester
	reqCh   chan incomingRequest
}

var _ rangesync.Requester = &fakeRequester{}

func newFakeRequester(
	t *testing.T,
	id p2p.Peer,
	handler server.StreamHandler,
	peers ...rangesync.Requester,
) *fakeRequester {
	fr := &fakeRequester{
		t:       t,
		id:      id,
		handler: handler,
		reqCh:   make(chan incomingRequest),
		peers:   make(map[p2p.Peer]*fakeRequester),
	}
	for _, p := range peers {
		pfr := p.(*fakeRequester)
		fr.peers[pfr.id] = pfr
	}
	return fr
}

func (fr *fakeRequester) Run(ctx context.Context) error {
	if fr.handler == nil {
		panic("no handler")
	}
	for {
		var req incomingRequest
		select {
		case <-ctx.Done():
			return nil
		case req = <-fr.reqCh:
		}
		if err := fr.handler(ctx, p2p.Peer(""), req.initialRequest, req.stream); err != nil {
			assert.Fail(fr.t, "handler error: %v", err)
		}
	}
}

func (fr *fakeRequester) StreamRequest(
	ctx context.Context,
	pid p2p.Peer,
	initialRequest []byte,
	callback server.StreamRequestCallback,
	extraProtocols ...string,
) error {
	p, found := fr.peers[pid]
	if !found {
		return fmt.Errorf("bad peer %q", pid)
	}
	rClient, wServer := io.Pipe()
	rServer, wClient := io.Pipe()
	for _, s := range []io.Closer{rClient, wClient, rServer, wServer} {
		defer s.Close()
	}
	clientStream := &pipeStream{ReadCloser: rClient, WriteCloser: wClient}
	serverStream := &pipeStream{ReadCloser: rServer, WriteCloser: wServer}
	select {
	case p.reqCh <- incomingRequest{
		initialRequest: initialRequest,
		stream:         serverStream,
	}:
	case <-ctx.Done():
		return ctx.Err()
	}
	return callback(ctx, clientStream)
}

func runRequester(tb testing.TB, r rangesync.Requester) context.Context {
	var eg errgroup.Group
	ctx, cancel := context.WithCancel(context.Background())
	eg.Go(func() error {
		return r.Run(ctx)
	})
	tb.Cleanup(func() {
		cancel()
		eg.Wait()
	})
	return ctx
}

func getMsgs(tb testing.TB, c rangesync.Conduit, n int) []rangesync.SyncMessage {
	msgs := make([]rangesync.SyncMessage, n)
	for i := 0; i < n; i++ {
		var err error
		msgs[i], err = c.NextMessage()
		require.NoError(tb, err)
	}
	return msgs
}

func TestWireConduit(t *testing.T) {
	hs := make([]rangesync.KeyBytes, 16)
	for n := range hs {
		hs[n] = rangesync.RandomKeyBytes(32)
	}
	fp := rangesync.Fingerprint(hs[2][:12])
	srv := newFakeRequester(
		t, "srv",
		func(ctx context.Context, _ p2p.Peer, initialRequest []byte, stream io.ReadWriter) error {
			require.Equal(t, []byte("hello"), initialRequest)
			c := rangesync.StartWireConduit(ctx, stream, rangesync.DefaultConfig())
			defer c.Stop()
			s := rangesync.Sender{c}
			require.Equal(t, []rangesync.SyncMessage{
				&rangesync.FingerprintMessage{
					RangeX:           rangesync.CHash(hs[0]),
					RangeY:           rangesync.CHash(hs[1]),
					RangeFingerprint: fp,
					NumItems:         4,
				},
				&rangesync.EndRoundMessage{},
			}, getMsgs(t, c, 2))
			require.NoError(t, s.SendRangeContents(hs[0], hs[3], 2))
			require.NoError(t, s.SendRangeContents(hs[3], hs[6], 2))
			require.NoError(t, s.SendChunk([]rangesync.KeyBytes{hs[4], hs[5], hs[7], hs[8]}))
			require.NoError(t, s.SendEndRound())
			require.Equal(t, []rangesync.SyncMessage{
				&rangesync.ItemBatchMessage{
					ContentKeys: rangesync.KeyCollection{
						Keys: []rangesync.KeyBytes{hs[9], hs[10], hs[11]},
					},
				},
				&rangesync.EndRoundMessage{},
			}, getMsgs(t, c, 2))
			require.NoError(t, s.SendDone())
			c.End()
			return nil
		})

	runRequester(t, srv)

	client := newFakeRequester(t, "client", nil, srv)
	require.NoError(t, client.StreamRequest(context.Background(), "srv", []byte("hello"),
		func(ctx context.Context, stream io.ReadWriter) error {
			c := rangesync.StartWireConduit(ctx, stream, rangesync.DefaultConfig())
			defer c.Stop()
			s := rangesync.Sender{c}
			require.NoError(t, s.SendFingerprint(hs[0], hs[1], fp, 4))
			require.NoError(t, s.SendEndRound())
			require.Equal(t, []rangesync.SyncMessage{
				&rangesync.RangeContentsMessage{
					RangeX:   rangesync.CHash(hs[0]),
					RangeY:   rangesync.CHash(hs[3]),
					NumItems: 2,
				},
				&rangesync.RangeContentsMessage{
					RangeX:   rangesync.CHash(hs[3]),
					RangeY:   rangesync.CHash(hs[6]),
					NumItems: 2,
				},
				&rangesync.ItemBatchMessage{
					ContentKeys: rangesync.KeyCollection{
						Keys: []rangesync.KeyBytes{hs[4], hs[5], hs[7], hs[8]},
					},
				},
				&rangesync.EndRoundMessage{},
			}, getMsgs(t, c, 4))
			require.NoError(t, s.SendChunk([]rangesync.KeyBytes{hs[9], hs[10], hs[11]}))
			require.NoError(t, s.SendEndRound())
			require.Equal(t, []rangesync.SyncMessage{
				&rangesync.DoneMessage{},
			}, getMsgs(t, c, 1))
			c.End()
			return nil
		}))
}

func TestWireConduit_Limits(t *testing.T) {
	for _, tc := range []struct {
		name         string
		trafficLimit int
		messageLimit int
		error        error
	}{
		{
			name:         "message limit hit",
			messageLimit: 10,
			error:        rangesync.ErrMessageLimitExceeded,
		},
		{
			name:         "traffic limit hit",
			trafficLimit: 100,
			error:        rangesync.ErrTrafficLimitExceeded,
		},
		{
			name:         "limits not hit",
			trafficLimit: 10000,
			messageLimit: 1000,
			error:        nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errCh := make(chan error)
			srv := newFakeRequester(
				t, "srv",
				func(
					ctx context.Context,
					_ p2p.Peer,
					initialRequest []byte,
					stream io.ReadWriter,
				) error {
					cfg := rangesync.DefaultConfig()
					cfg.TrafficLimit = tc.trafficLimit
					cfg.MessageLimit = tc.messageLimit
					c := rangesync.StartWireConduit(ctx, stream, cfg)
					defer c.Stop()
					for range 11 {
						msg, err := c.NextMessage()
						if err != nil {
							errCh <- err
							return nil
						}
						if msg == nil {
							break
						}
					}
					errCh <- nil
					s := rangesync.Sender{c}
					return s.SendDone()
				})

			runRequester(t, srv)

			client := newFakeRequester(t, "client", nil, srv)
			var eg errgroup.Group
			ctx, cancel := context.WithCancel(context.Background())
			defer func() {
				cancel()
				eg.Wait()
			}()
			eg.Go(func() error {
				client.StreamRequest(ctx, "srv", []byte("hello"),
					func(ctx context.Context, stream io.ReadWriter) error {
						c := rangesync.StartWireConduit(
							ctx, stream, rangesync.DefaultConfig())
						defer c.Stop()
						s := rangesync.Sender{c}
						for i := 0; i < 11; i++ {
							s.SendFingerprint(
								rangesync.RandomKeyBytes(32),
								rangesync.RandomKeyBytes(32),
								rangesync.Fingerprint{}, 1)
						}
						c.NextMessage()
						return nil
					})
				return nil
			})

			if tc.error != nil {
				require.ErrorIs(t, <-errCh, tc.error)
			} else {
				require.NoError(t, <-errCh)
			}
		})
	}
}

func TestWireConduit_StopSend(t *testing.T) {
	started := make(chan struct{})
	srv := newFakeRequester(
		t, "srv",
		func(ctx context.Context, _ p2p.Peer, initialRequest []byte, stream io.ReadWriter) error {
			close(started)
			// This will hang
			<-ctx.Done()
			return nil
		})

	runRequester(t, srv)

	client := newFakeRequester(t, "client", nil, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client.StreamRequest(ctx, "srv", []byte("hello"),
		func(ctx context.Context, stream io.ReadWriter) error {
			c := rangesync.StartWireConduit(ctx, stream, rangesync.DefaultConfig())
			s := rangesync.Sender{c}
			// The actual message is enqueued but not sent
			s.SendDone()
			select {
			case <-ctx.Done():
			case <-started:
			}
			c.Stop() // stop the sender and wait for it to terminate
			return nil
		})
	require.NoError(t, ctx.Err(), "the context should not be canceled")
}
