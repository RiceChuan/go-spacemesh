package atxsdata

import (
	"slices"
	"sync"
	"sync/atomic"

	"golang.org/x/exp/maps"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/signing"
)

// SAFETY: all exported fields are read-only and are safe to read concurrently.
// Thanks to the fact that ATX is immutable, it is safe to return a pointer to it.
type ATX struct {
	Node               types.NodeID
	Coinbase           types.Address
	Weight             uint64
	BaseHeight, Height uint64
	Nonce              types.VRFPostIndex
}

func New() *Data {
	return &Data{
		malicious: map[types.NodeID]struct{}{},
		epochs:    map[types.EpochID]map[types.ATXID]*ATX{},

		signers: map[types.NodeID]struct{}{},
		managed: map[types.EpochID]map[types.NodeID]types.ATXID{},
	}
}

type Data struct {
	evicted atomic.Uint32

	mu        sync.RWMutex
	malicious map[types.NodeID]struct{}
	epochs    map[types.EpochID]map[types.ATXID]*ATX

	signers map[types.NodeID]struct{}
	managed map[types.EpochID]map[types.NodeID]types.ATXID
}

func (d *Data) Register(sig *signing.EdSigner) {
	if _, exists := d.signers[sig.NodeID()]; exists {
		return
	}
	d.signers[sig.NodeID()] = struct{}{}

	// update quick access for newly registered signer
	for epoch, ecache := range d.epochs {
		for id, atx := range ecache {
			if atx.Node == sig.NodeID() {
				d.managed[epoch][sig.NodeID()] = id
			}
		}
	}
}

func (d *Data) Evicted() types.EpochID {
	return types.EpochID(d.evicted.Load())
}

func (d *Data) IsEvicted(epoch types.EpochID) bool {
	return d.evicted.Load() >= epoch.Uint32()
}

// EvictEpoch is a notification for cache to evict epochs that are not useful
// to keep in memory.
func (d *Data) EvictEpoch(evict types.EpochID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.IsEvicted(evict) {
		return
	}
	if d.evicted.Load() < evict.Uint32() {
		d.evicted.Store(evict.Uint32())
	}
	for epoch := range d.managed {
		if epoch <= evict {
			delete(d.managed, epoch)
		}
	}
	for epoch := range d.epochs {
		if epoch <= evict {
			delete(d.epochs, epoch)
			atxsCounter.DeleteLabelValues(epoch.String())
		}
	}
}

// AddFromHeader extracts relevant fields from an ActivationTx and adds them together with nonce and malicious flag.
// Returns the ATX that was added to the store (if any) or `nil` if it wasn't.
func (d *Data) AddFromAtx(atx *types.ActivationTx, malicious bool) *ATX {
	return d.Add(
		atx.TargetEpoch(),
		atx.SmesherID,
		atx.Coinbase,
		atx.ID(),
		atx.Weight,
		atx.BaseTickHeight,
		atx.TickHeight(),
		atx.VRFNonce,
		malicious,
	)
}

// Add adds ATX data to the store.
// Returns whether the ATX was added to the store.
func (d *Data) AddAtx(target types.EpochID, id types.ATXID, atx *ATX) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.IsEvicted(target) {
		return false
	}
	ecache, exists := d.epochs[target]
	if !exists {
		d.epochs[target] = map[types.ATXID]*ATX{}
		ecache = d.epochs[target]
	}
	if _, exists = ecache[id]; exists {
		return false
	}

	atxsCounter.WithLabelValues(target.String()).Inc()
	ecache[id] = atx
	if _, exists = d.signers[atx.Node]; exists {
		managedCache, exists := d.managed[target]
		if !exists {
			d.managed[target] = map[types.NodeID]types.ATXID{}
			managedCache = d.managed[target]
		}
		managedCache[atx.Node] = id
	}
	return true
}

// Add adds ATX data to the store.
// Returns the ATX that was added to the store (if any) or `nil` if it wasn't.
func (d *Data) Add(
	epoch types.EpochID,
	node types.NodeID,
	coinbase types.Address,
	atxid types.ATXID,
	weight, baseHeight, height uint64,
	nonce types.VRFPostIndex,
	malicious bool,
) *ATX {
	atx := &ATX{
		Node:       node,
		Coinbase:   coinbase,
		Weight:     weight,
		BaseHeight: baseHeight,
		Height:     height,
		Nonce:      nonce,
	}
	if malicious {
		d.SetMalicious(node)
	}
	if d.AddAtx(epoch, atxid, atx) {
		return atx
	}
	return nil
}

func (d *Data) IsMalicious(node types.NodeID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.malicious[node]
	return exists
}

func (d *Data) SetMalicious(node types.NodeID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.malicious[node] = struct{}{}
}

func (d *Data) MaliciousIdentities() []types.NodeID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return maps.Keys(d.malicious)
}

// Get returns atx data.
// SAFETY: The returned pointer MUST NOT be modified.
func (d *Data) Get(epoch types.EpochID, atx types.ATXID) *ATX {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ecache, exists := d.epochs[epoch]
	if !exists {
		return nil
	}
	data, exists := ecache[atx]
	if !exists {
		return nil
	}
	return data
}

// GetByEpochAndNodeID returns atx data by epoch and node id. This query will be slow for nodeIDs that
// are not managed by the node.
func (d *Data) GetByEpochAndNodeID(epoch types.EpochID, node types.NodeID) (types.ATXID, *ATX) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	atxcache, exists := d.managed[epoch]
	if exists {
		if atxid, exists := atxcache[node]; exists {
			atx, exists := d.epochs[epoch][atxid]
			if exists {
				return atxid, atx
			}
		}
	}

	ecache, exists := d.epochs[epoch]
	if !exists {
		return types.EmptyATXID, nil
	}
	for id, atx := range ecache {
		if atx.Node == node {
			return id, atx
		}
	}
	return types.EmptyATXID, nil
}

func (d *Data) Size(target types.EpochID) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ecache, exists := d.epochs[target]
	if !exists {
		return 0
	}
	return len(ecache)
}

type lockGuard struct{}

// AtxFilter is a function that filters atxs.
// The `lockGuard` prevents using the filter functions outside of the allowed context
// to prevent data races.
type AtxFilter func(*Data, *ATX, lockGuard) bool

func NotMalicious(d *Data, atx *ATX, _ lockGuard) bool {
	_, m := d.malicious[atx.Node]
	return !m
}

// IterateInEpoch calls `fn` for every ATX in epoch.
// If filters are provided, only atxs that pass all filters are returned.
// SAFETY: The returned pointer MUST NOT be modified.
func (d *Data) IterateInEpoch(epoch types.EpochID, fn func(types.ATXID, *ATX), filters ...AtxFilter) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ecache, exists := d.epochs[epoch]
	if !exists {
		return
	}
	for id, atx := range ecache {
		ok := true
		for _, filter := range filters {
			ok = ok && filter(d, atx, lockGuard{})
		}
		if ok {
			fn(id, atx)
		}
	}
}

func (d *Data) IterateHighTicksInEpoch(target types.EpochID, fn func(types.ATXID) bool) {
	type candidate struct {
		id types.ATXID
		*ATX
	}
	candidates := make([]candidate, 0, d.Size(target))
	d.IterateInEpoch(target, func(id types.ATXID, atx *ATX) {
		candidates = append(candidates, candidate{id: id, ATX: atx})
	}, NotMalicious)

	slices.SortFunc(candidates, func(a, b candidate) int {
		switch {
		case a.Height < b.Height:
			return 1
		case a.Height > b.Height:
			return -1
		}
		return 0
	})

	for _, c := range candidates {
		if cont := fn(c.id); !cont {
			return
		}
	}
}

func (d *Data) MissingInEpoch(epoch types.EpochID, atxs []types.ATXID) []types.ATXID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ecache, exists := d.epochs[epoch]
	if !exists {
		return atxs
	}
	var missing []types.ATXID
	for _, id := range atxs {
		if _, exists := ecache[id]; !exists {
			missing = append(missing, id)
		}
	}
	return missing
}

// WeightForSet computes total weight of atxs in the set and returned array with
// atxs in the set that weren't used.
func (d *Data) WeightForSet(epoch types.EpochID, set []types.ATXID) (uint64, []bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ecache, exists := d.epochs[epoch]

	// TODO(dshulyak) bitfield is a perfect fit here
	used := make([]bool, len(set))
	if !exists {
		return 0, used
	}
	var weight uint64
	for i, id := range set {
		if data, exists := ecache[id]; exists {
			weight += data.Weight
			used[i] = true
		}
	}
	return weight, used
}
