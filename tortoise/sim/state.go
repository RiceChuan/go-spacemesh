package sim

import (
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/spacemeshos/go-spacemesh/atxsdata"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/beacons"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/certificates"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
)

func newState(tb testing.TB, logger *zap.Logger, conf config, atxdata *atxsdata.Data) State {
	return State{
		logger:  logger,
		DB:      newCacheDB(tb, logger, conf),
		Atxdata: atxdata,
	}
}

// State of the node.
type State struct {
	logger *zap.Logger

	DB      *datastore.CachedDB
	Atxdata *atxsdata.Data
}

// OnBeacon callback to store generated beacon.
func (s *State) OnBeacon(eid types.EpochID, beacon types.Beacon) {
	if err := beacons.Add(s.DB, eid+1, beacon); err != nil {
		s.logger.Panic("failed to add beacon", zap.Error(err))
	}
}

// OnActivationTx callback to store activation transaction.
func (s *State) OnActivationTx(atx *types.ActivationTx) {
	// TODO: consider using actual values for malicious if needed
	s.Atxdata.AddFromAtx(atx, false)
	if err := atxs.Add(s.DB, atx, types.AtxBlob{}); err != nil {
		s.logger.Panic("failed to add atx", zap.Error(err))
	}
}

// OnBallot callback to store ballot.
func (s *State) OnBallot(ballot *types.Ballot) {
	exist, _ := ballots.Get(s.DB, ballot.ID())
	if exist != nil {
		return
	}
	if err := ballots.Add(s.DB, ballot); err != nil {
		s.logger.Panic("failed to save ballot", zap.Error(err))
	}
}

// OnBlock callback to store block.
func (s *State) OnBlock(block *types.Block) {
	exist, _ := blocks.Get(s.DB, block.ID())
	if exist != nil {
		return
	}

	if err := blocks.Add(s.DB, block); err != nil && !errors.Is(err, sql.ErrObjectExists) {
		s.logger.Panic("failed to save block", zap.Error(err))
	}
}

// OnHareOutput callback to store hare output.
func (s *State) OnHareOutput(lid types.LayerID, bid types.BlockID) {
	if err := certificates.SetHareOutput(s.DB, lid, bid); err != nil {
		s.logger.Panic("failed to save hare output", zap.Error(err))
	}
}

// OnCoinflip callback to store coinflip.
func (s *State) OnCoinflip(lid types.LayerID, coinflip bool) {
	if err := layers.SetWeakCoin(s.DB, lid, coinflip); err != nil {
		s.logger.Panic("failed to save coin flip", zap.Error(err))
	}
}
