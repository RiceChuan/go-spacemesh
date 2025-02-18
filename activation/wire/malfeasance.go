package wire

import (
	"context"

	"github.com/spacemeshos/go-scale"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

//go:generate scalegen

// MerkleTreeIndex is the index of the leaf containing the given field in the merkle tree.
type MerkleTreeIndex uint64

const (
	PublishEpochIndex MerkleTreeIndex = iota
	PositioningATXIndex
	CoinbaseIndex
	InitialPostRootIndex
	PreviousATXsRootIndex
	NIPostsRootIndex
	VRFNonceIndex
	MarriagesRootIndex
	MarriageATXIndex
)

type InitialPostTreeIndex uint64

const (
	CommitmentATXIndex InitialPostTreeIndex = iota
	InitialPostIndex
)

type NIPostTreeIndex uint64

const (
	MembershipIndex NIPostTreeIndex = iota
	ChallengeIndex
	PostsRootIndex
)

type MarriageCertificateIndex uint64

const (
	ReferenceATXIndex MarriageCertificateIndex = iota
	SignatureIndex
)

type SubPostTreeIndex uint64

const (
	MarriageIndex SubPostTreeIndex = iota
	PrevATXIndex
	MembershipLeafIndex
	PostIndex
	NumUnitsIndex
)

// ProofType is an identifier for the type of proof that is encoded in the ATXProof.
type ProofType byte

const (
	// TODO(mafa): legacy types for future migration to new malfeasance proofs.
	LegacyDoublePublish  ProofType = 0x00
	LegacyInvalidPost    ProofType = 0x01
	LegacyInvalidPrevATX ProofType = 0x02

	DoubleMarry     ProofType = 0x10
	DoubleMerge     ProofType = 0x11
	InvalidPost     ProofType = 0x12
	InvalidPrevious ProofType = 0x13
)

// ProofVersion is an identifier for the version of the proof that is encoded in the ATXProof.
type ProofVersion byte

type ATXProof struct {
	// Version is the version identifier of the proof. This can be used to extend the ATX proof in the future.
	Version ProofVersion
	// ProofType is the type of proof that is being provided.
	ProofType ProofType
	// Proof is the actual proof. Its type depends on the ProofType.
	Proof []byte `scale:"max=1048576"` // max size of proof is 1MiB
}

// Proof is an interface for all types of proofs that can be provided in an ATXProof.
// Generally the proof should be able to validate itself and be scale encoded.
type Proof interface {
	scale.Encodable

	Valid(ctx context.Context, malHandler MalfeasanceValidator) (types.NodeID, error)
}
