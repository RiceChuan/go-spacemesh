package types

import (
	"encoding/hex"

	"github.com/spacemeshos/go-spacemesh/common/util"
)

const (
	// BeaconSize in bytes.
	BeaconSize = 4
)

// Beacon defines the beacon value. A beacon is generated once per epoch and is used to
// - verify smesher's VRF signature for proposal/ballot eligibility
// - determine good ballots in verifying tortoise.
type Beacon [BeaconSize]byte

// EmptyBeacon is a canonical empty Beacon.
var EmptyBeacon = Beacon{}

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (b Beacon) String() string { return hex.EncodeToString(b[:]) }

// Bytes gets the byte representation of the underlying hash.
func (b Beacon) Bytes() []byte {
	return b[:]
}

func (b *Beacon) MarshalText() ([]byte, error) {
	return util.Base64Encode(b[:]), nil
}

func (b *Beacon) UnmarshalText(buf []byte) error {
	return util.Base64Decode(b[:], buf)
}

// BytesToBeacon sets the first BeaconSize bytes of b to the Beacon's data.
func BytesToBeacon(b []byte) Beacon {
	var beacon Beacon
	copy(beacon[:], b)
	return beacon
}

// HexToBeacon sets byte representation of s to a Beacon.
func HexToBeacon(s string) Beacon {
	return BytesToBeacon(util.FromHex(s))
}
