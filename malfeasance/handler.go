package malfeasance

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/events"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/malfeasance/wire"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/identities"
)

var (
	errKnownProof    = errors.New("known proof")
	errMalformedData = fmt.Errorf("%w: malformed data", pubsub.ErrValidationReject)
	errWrongHash     = fmt.Errorf("%w: incorrect hash", pubsub.ErrValidationReject)
	errUnknownProof  = fmt.Errorf("%w: unknown proof type", pubsub.ErrValidationReject)
	errInvalidProof  = fmt.Errorf("%w: invalid proof", pubsub.ErrValidationReject)
)

type MalfeasanceType byte

const (
	MultipleATXs     MalfeasanceType = MalfeasanceType(wire.MultipleATXs)
	MultipleBallots                  = MalfeasanceType(wire.MultipleBallots)
	HareEquivocation                 = MalfeasanceType(wire.HareEquivocation)
	InvalidPostIndex                 = MalfeasanceType(wire.InvalidPostIndex)
	InvalidPrevATX                   = MalfeasanceType(wire.InvalidPrevATX)
)

// Handler processes MalfeasanceProof from gossip and, if deems it valid, propagates it to peers.
type Handler struct {
	logger   *zap.Logger
	cdb      *datastore.CachedDB
	self     p2p.Peer
	nodeIDs  []types.NodeID
	tortoise tortoise

	handlers map[MalfeasanceType]MalfeasanceHandler
}

func NewHandler(
	cdb *datastore.CachedDB,
	lg *zap.Logger,
	self p2p.Peer,
	nodeIDs []types.NodeID,
	tortoise tortoise,
) *Handler {
	return &Handler{
		logger:   lg,
		cdb:      cdb,
		self:     self,
		nodeIDs:  nodeIDs,
		tortoise: tortoise,

		handlers: make(map[MalfeasanceType]MalfeasanceHandler),
	}
}

func (h *Handler) RegisterHandler(malfeasanceType MalfeasanceType, handler MalfeasanceHandler) {
	if _, ok := h.handlers[malfeasanceType]; ok {
		h.logger.Panic("handler already registered", zap.Int("malfeasanceType", int(malfeasanceType)))
	}

	h.handlers[malfeasanceType] = handler
}

func (h *Handler) reportMalfeasance(smesher types.NodeID) {
	h.tortoise.OnMalfeasance(smesher)
	events.ReportMalfeasance(smesher)
	if slices.Contains(h.nodeIDs, smesher) {
		events.EmitOwnMalfeasanceProof(smesher)
	}
}

func (h *Handler) countProof(mp *wire.MalfeasanceProof) {
	h.handlers[MalfeasanceType(mp.Proof.Type)].ReportProof(numProofs)
}

func (h *Handler) countInvalidProof(p *wire.MalfeasanceProof) {
	h.handlers[MalfeasanceType(p.Proof.Type)].ReportInvalidProof(numInvalidProofs)
}

func (h *Handler) Info(ctx context.Context, nodeID types.NodeID) (map[string]string, error) {
	var blob sql.Blob
	if err := identities.LoadMalfeasanceBlob(ctx, h.cdb, nodeID.Bytes(), &blob); err != nil {
		return nil, fmt.Errorf("load malfeasance proof: %w", err)
	}

	var p wire.MalfeasanceProof
	if err := codec.Decode(blob.Bytes, &p); err != nil {
		return nil, fmt.Errorf("decode malfeasance proof: %w", err)
	}
	mh, ok := h.handlers[MalfeasanceType(p.Proof.Type)]
	if !ok {
		return nil, fmt.Errorf("unknown malfeasance type %d", p.Proof.Type)
	}
	properties, err := mh.Info(p.Proof.Data)
	if err != nil {
		return nil, fmt.Errorf("malfeasance info: %w", err)
	}
	properties["domain"] = "0" // for malfeasance V1 there are no domains
	properties["type"] = strconv.FormatUint(uint64(p.Proof.Type), 10)
	return properties, nil
}

// HandleSyncedMalfeasanceProof is the sync validator for MalfeasanceProof.
func (h *Handler) HandleSyncedMalfeasanceProof(
	ctx context.Context,
	expHash types.Hash32,
	peer p2p.Peer,
	data []byte,
) error {
	var p wire.MalfeasanceProof
	if err := codec.Decode(data, &p); err != nil {
		numMalformed.Inc()
		h.logger.Error("malformed message (sync)", log.ZContext(ctx), zap.Error(err))
		return errMalformedData
	}
	nodeID, err := h.validateAndSave(ctx, &p)
	if err == nil && types.Hash32(nodeID) != expHash {
		// we log & return because libp2p will ignore the message if we return an error,
		// but only log "validation ignored" instead of the error we return
		h.logger.Warn("malfeasance proof for wrong identity",
			log.ZContext(ctx),
			zap.Stringer("peer", peer),
			log.ZShortStringer("expected", expHash),
			log.ZShortStringer("got", nodeID),
		)
		return fmt.Errorf(
			"%w: malfeasance proof want %s, got %s",
			errWrongHash,
			expHash.ShortString(),
			nodeID.ShortString(),
		)
	}
	if errors.Is(err, errKnownProof) {
		return nil
	}
	return err
}

// HandleMalfeasanceProof is the gossip receiver for MalfeasanceGossip.
func (h *Handler) HandleMalfeasanceProof(ctx context.Context, peer p2p.Peer, data []byte) error {
	var p wire.MalfeasanceGossip
	if err := codec.Decode(data, &p); err != nil {
		numMalformed.Inc()
		h.logger.Error("malformed message", log.ZContext(ctx), zap.Error(err))
		return errMalformedData
	}
	if p.Eligibility != nil {
		numMalformed.Inc()
		return fmt.Errorf("%w: eligibility field was deprecated with hare3", pubsub.ErrValidationReject)
	}
	if peer == h.self {
		id, err := h.Validate(ctx, &p.MalfeasanceProof)
		if err != nil {
			h.logger.Warn("malfeasance proof failed validation during publish",
				log.ZContext(ctx),
				zap.Inline(&p.MalfeasanceProof),
				zap.Error(err),
			)
			h.countInvalidProof(&p.MalfeasanceProof)
			return fmt.Errorf("%w: %s", pubsub.ErrValidationReject, err)
		}
		h.reportMalfeasance(id)
		// node saves malfeasance proof eagerly/atomically with the malicious data.
		// it has validated the proof before saving to db.
		h.countProof(&p.MalfeasanceProof)
		return nil
	}
	_, err := h.validateAndSave(ctx, &p.MalfeasanceProof)
	if errors.Is(err, errKnownProof) {
		return nil
	}
	return err
}

func (h *Handler) validateAndSave(ctx context.Context, p *wire.MalfeasanceProof) (types.NodeID, error) {
	p.SetReceived(time.Now())
	nodeID, err := h.Validate(ctx, p)
	switch {
	case errors.Is(err, errUnknownProof):
		numMalformed.Inc()
		return types.EmptyNodeID, err
	case err != nil:
		h.countInvalidProof(p)
		return types.EmptyNodeID, errors.Join(err, pubsub.ErrValidationReject)
	}
	proofBytes := codec.MustEncode(p)
	if err := h.cdb.WithTxImmediate(ctx, func(dbtx sql.Transaction) error {
		malicious, err := identities.IsMalicious(dbtx, nodeID)
		if err != nil {
			return fmt.Errorf("check known malicious: %w", err)
		}
		if malicious {
			h.logger.Debug("known malicious identity", log.ZContext(ctx), zap.Stringer("smesher", nodeID))
			return errKnownProof
		}
		if err := identities.SetMalicious(dbtx, nodeID, proofBytes, time.Now()); err != nil {
			return fmt.Errorf("add malfeasance proof: %w", err)
		}
		return nil
	}); err != nil {
		if !errors.Is(err, errKnownProof) {
			h.logger.Error("failed to save MalfeasanceProof",
				log.ZContext(ctx),
				zap.Stringer("smesher", nodeID),
				zap.Inline(p),
				zap.Error(err),
			)
		}
		return nodeID, err
	}
	h.reportMalfeasance(nodeID)
	h.cdb.CacheMalfeasanceProof(nodeID, proofBytes)
	h.countProof(p)
	h.logger.Debug("new malfeasance proof",
		log.ZContext(ctx),
		zap.Stringer("smesher", nodeID),
		zap.Inline(p),
	)
	return nodeID, nil
}

func (h *Handler) Validate(ctx context.Context, p *wire.MalfeasanceProof) (types.NodeID, error) {
	mh, ok := h.handlers[MalfeasanceType(p.Proof.Type)]
	if !ok {
		return types.EmptyNodeID, fmt.Errorf("%w: unknown malfeasance type", errUnknownProof)
	}

	nodeID, err := mh.Validate(ctx, p.Proof.Data)
	if err == nil {
		return nodeID, nil
	}
	return nodeID, fmt.Errorf("%w: %v", errInvalidProof, err)
}
