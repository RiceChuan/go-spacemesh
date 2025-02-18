package sync2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/fetch/peers"
	"github.com/spacemeshos/go-spacemesh/sync2/multipeer"
	"github.com/spacemeshos/go-spacemesh/sync2/rangesync"
)

// Config contains the configuration for the P2PHashSync.
type Config struct {
	rangesync.RangeSetReconcilerConfig  `mapstructure:",squash"`
	multipeer.MultiPeerReconcilerConfig `mapstructure:",squash"`
	TrafficLimit                        int           `mapstructure:"traffic-limit"`
	MessageLimit                        int           `mapstructure:"message-limit"`
	MaxDepth                            uint          `mapstructure:"max-depth"`
	BatchSize                           uint          `mapstructure:"batch-size"`
	MaxAttempts                         uint          `mapstructure:"max-attempts"`
	MaxBatchRetries                     uint          `mapstructure:"max-batch-retries"`
	FailedBatchDelay                    time.Duration `mapstructure:"failed-batch-delay"`
	AdvanceInterval                     time.Duration `mapstructure:"advance-interval"`
}

func (cfg *Config) Validate(logger *zap.Logger) bool {
	r := cfg.RangeSetReconcilerConfig.Validate(logger)
	// always invoke Validate to log validation errors
	r = cfg.MultiPeerReconcilerConfig.Validate(logger) && r
	if cfg.MaxDepth < 1 {
		logger.Error("max-depth must be at least 1")
		r = false
	}
	if cfg.BatchSize < 1 {
		logger.Error("batch-size must be at least 1")
		r = false
	}
	if cfg.MaxAttempts < 1 {
		logger.Error("max-attempts must be at least 1")
		r = false
	}
	if cfg.AdvanceInterval <= 0 {
		logger.Error("advance-interval must be positive")
		r = false
	}
	return r
}

// DefaultConfig returns the default configuration for the P2PHashSync.
func DefaultConfig() Config {
	return Config{
		RangeSetReconcilerConfig:  rangesync.DefaultConfig(),
		MultiPeerReconcilerConfig: multipeer.DefaultConfig(),
		TrafficLimit:              200_000_000,
		MessageLimit:              20_000_000,
		MaxDepth:                  24,
		BatchSize:                 1000,
		MaxAttempts:               3,
		MaxBatchRetries:           3,
		FailedBatchDelay:          10 * time.Second,
		AdvanceInterval:           1 * time.Minute,
	}
}

// P2PHashSync is handles the synchronization of a local OrderedSet against other peers.
type P2PHashSync struct {
	logger           *zap.Logger
	cfg              Config
	enableActiveSync bool
	os               rangesync.OrderedSet
	syncBase         multipeer.SyncBase
	reconciler       *multipeer.MultiPeerReconciler
	cancel           context.CancelFunc
	eg               errgroup.Group
	startOnce        sync.Once
	running          atomic.Bool
	kickCh           chan struct{}
}

// NewP2PHashSync creates a new P2PHashSync.
func NewP2PHashSync(
	logger *zap.Logger,
	d *rangesync.Dispatcher,
	name string,
	os rangesync.OrderedSet,
	keyLen int,
	peers *peers.Peers,
	handler multipeer.SyncKeyHandler,
	cfg Config,
	enableActiveSync bool,
) (*P2PHashSync, error) {
	if !cfg.Validate(logger) {
		return nil, errors.New("invalid config")
	}
	s := &P2PHashSync{
		logger:           logger,
		os:               os,
		cfg:              cfg,
		kickCh:           make(chan struct{}, 1),
		enableActiveSync: enableActiveSync,
	}
	ps := rangesync.NewPairwiseSetSyncer(logger, d, name, cfg.RangeSetReconcilerConfig)
	s.syncBase = multipeer.NewSetSyncBase(ps, s.os, handler)
	s.reconciler = multipeer.NewMultiPeerReconciler(
		logger, cfg.MultiPeerReconcilerConfig,
		s.syncBase, peers, keyLen, int(cfg.MaxDepth))
	d.Register(name, s.syncBase.Serve)
	return s, nil
}

// Set returns the OrderedSet that is being synchronized.
func (s *P2PHashSync) Set() rangesync.OrderedSet {
	return s.os
}

// Load loads the OrderedSet from the underlying storage.
func (s *P2PHashSync) Load() error {
	if s.os.Loaded() {
		return nil
	}
	s.logger.Info("loading the set")
	start := time.Now()
	// We pre-load the set to avoid waiting for it to load during a
	// sync request
	if err := s.os.EnsureLoaded(); err != nil {
		return fmt.Errorf("load set: %w", err)
	}
	info, err := s.os.GetRangeInfo(nil, nil)
	if err != nil {
		return fmt.Errorf("get range info: %w", err)
	}
	s.logger.Info("done loading the set",
		zap.Duration("elapsed", time.Since(start)),
		zap.Int("count", info.Count),
		zap.Stringer("fingerprint", info.Fingerprint),
		zap.Uint("maxDepth", s.cfg.MaxDepth))
	return nil
}

func (s *P2PHashSync) start() (isWaiting bool) {
	s.running.Store(true)
	isWaiting = true
	s.startOnce.Do(func() {
		isWaiting = false
		var ctx context.Context
		ctx, s.cancel = context.WithCancel(context.Background())
		if s.enableActiveSync {
			s.eg.Go(func() error {
				defer s.running.Store(false)
				return s.reconciler.Run(ctx, s.kickCh)
			})
		} else {
			s.logger.Info("active syncv2 is disabled")
		}
		s.eg.Go(func() error {
			ticker := time.NewTicker(s.cfg.AdvanceInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					s.logger.Debug("advancing OrderedSet on timer")
					if err := s.os.Advance(); err != nil {
						s.logger.Error("error advancing the set", zap.Error(err))
					}
				}
			}
		})
	})
	return isWaiting
}

// Start starts the multi-peer reconciler if it is not already running.
func (s *P2PHashSync) Start() {
	s.start()
}

// StartAndSync starts the multi-peer reconciler if it is not already running, and waits
// until the local OrderedSet is in sync with the peers.
func (s *P2PHashSync) StartAndSync(ctx context.Context) error {
	if s.start() {
		// If the multipeer reconciler is waiting for sync, we kick it to start
		// the sync so as not to wait for the next scheduled sync interval.
		s.kickCh <- struct{}{}
	}
	return s.WaitForSync(ctx)
}

// Stop stops the multi-peer reconciler.
func (s *P2PHashSync) Stop() {
	if !s.running.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if err := s.eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("P2PHashSync terminated with an error", zap.Error(err))
	}
}

// Synced returns true if the local OrderedSet is in sync with the peers, as determined by
// the multi-peer reconciler.
func (s *P2PHashSync) Synced() bool {
	return s.reconciler.Synced()
}

var errStopped = errors.New("syncer stopped")

// WaitForSync waits until the local OrderedSet is in sync with the peers.
func (s *P2PHashSync) WaitForSync(ctx context.Context) error {
	for !s.Synced() {
		if !s.running.Load() {
			return errStopped
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}
