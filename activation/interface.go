package activation

import (
	"context"
	"errors"
	"io"
	"net/url"
	"time"

	"github.com/spacemeshos/post/shared"
	"github.com/spacemeshos/post/verifying"

	"github.com/spacemeshos/go-spacemesh/activation/wire"
	"github.com/spacemeshos/go-spacemesh/common/types"
	mwire "github.com/spacemeshos/go-spacemesh/malfeasance/wire"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql/localsql/certifier"
	"github.com/spacemeshos/go-spacemesh/sql/localsql/nipost"
)

//go:generate mockgen -typed -package=activation -destination=./mocks.go -source=./interface.go

type atxReceiver interface {
	OnAtx(*types.ActivationTx)
}

type PostVerifier interface {
	io.Closer
	Verify(ctx context.Context, p *shared.Proof, m *shared.ProofMetadata, opts ...postVerifierOptionFunc) error
}

type scaler interface {
	scale(int)
}

type postVerifierCallOption struct {
	prioritized     bool
	verifierOptions []verifying.OptionFunc
}

type postVerifierOptionFunc func(*postVerifierCallOption)

func applyOptions(options ...postVerifierOptionFunc) postVerifierCallOption {
	opts := postVerifierCallOption{}
	for _, opt := range options {
		opt(&opts)
	}
	return opts
}

func PrioritizedCall() postVerifierOptionFunc {
	return func(o *postVerifierCallOption) {
		o.prioritized = true
	}
}

func WithVerifierOptions(ops ...verifying.OptionFunc) postVerifierOptionFunc {
	return func(o *postVerifierCallOption) {
		o.verifierOptions = ops
	}
}

// validatorOption is a functional option type for the validator.
type validatorOption func(*validatorOptions)

type nipostValidator interface {
	nipostValidatorV1
	nipostValidatorV2

	// VerifyChain fully verifies all dependencies of the given ATX and the ATX itself.
	VerifyChain(ctx context.Context, id, goldenATXID types.ATXID, opts ...VerifyChainOption) error
}

type layerClock interface {
	AwaitLayer(layerID types.LayerID) <-chan struct{}
	CurrentLayer() types.LayerID
	LayerToTime(types.LayerID) time.Time
}

type nipostBuilder interface {
	BuildNIPost(
		ctx context.Context,
		sig *signing.EdSigner,
		challengeHash types.Hash32,
		postChallenge *types.NIPostChallenge,
	) (*nipost.NIPostState, error)
	Proof(ctx context.Context, nodeID types.NodeID, challenge []byte, postChallenge *types.NIPostChallenge,
	) (*types.Post, *types.PostInfo, error)
	ResetState(types.NodeID) error
}

type syncer interface {
	RegisterForATXSynced() <-chan struct{}
}

// legacyMalfeasancePublisher is an interface for publishing legacy malfeasance proofs.
//
// It is used int he ATXv1 handler and will be replaced in the future by the atxMalfeasancePublisher, which will
// wrap legacy proofs into the new encoding structure.
type legacyMalfeasancePublisher interface {
	PublishProof(ctx context.Context, smesherID types.NodeID, proof *mwire.MalfeasanceProof) error
}

// atxMalfeasancePublisher is an interface for publishing atx malfeasance proofs.
//
// It encapsulates a specific malfeasance proof into a generic ATX malfeasance proof and publishes it by calling
// the underlying malfeasancePublisher.
type atxMalfeasancePublisher interface {
	Publish(ctx context.Context, nodeID types.NodeID, proof wire.Proof) error
}

type atxProvider interface {
	GetAtx(id types.ATXID) (*types.ActivationTx, error)
}

// PostSetupProvider defines the functionality required for Post setup.
// This interface is used by the atx builder and currently implemented by the PostSetupManager.
// Eventually most of the functionality will be moved to the PoSTClient.
type postSetupProvider interface {
	PrepareInitializer(ctx context.Context, opts PostSetupOpts, id types.NodeID) error
	StartSession(context context.Context, id types.NodeID) error
	Status() *PostSetupStatus
	Reset() error
}

// SmeshingProvider defines the functionality required for the node's Smesher API.
type SmeshingProvider interface {
	Smeshing() bool
	StartSmeshing(types.Address) error
	StopSmeshing(bool) error
	SmesherIDs() []types.NodeID
	Coinbase() types.Address
	SetCoinbase(coinbase types.Address)
}

// PoetService servers as an interface to communicate with a PoET server.
// It is used to submit challenges and fetch proofs.
type PoetService interface {
	Address() string

	// Submit registers a challenge in the proving service current open round.
	Submit(
		ctx context.Context,
		deadline time.Time,
		prefix, challenge []byte,
		signature types.EdSignature,
		nodeID types.NodeID,
	) (*types.PoetRound, error)

	// Certify requests a certificate for the given nodeID.
	//
	// Returns ErrCertificatesNotSupported if the service does not support certificates.
	Certify(ctx context.Context, id types.NodeID) (*certifier.PoetCert, error)

	// Proof returns the proof for the given round ID.
	Proof(ctx context.Context, roundID string) (*types.PoetProof, []types.Hash32, error)

	// TickSize returns tickSize configured for particular PoET service
	TickSize() uint64
}

// A certifier client that the certifierService uses to obtain certificates
// The implementation can use any method to obtain the certificate,
// for example, POST verification.
type certifierClient interface {
	// Certify obtains a certificate in a remote certifier service.
	Certify(
		ctx context.Context,
		id types.NodeID,
		certifierAddress *url.URL,
		pubkey []byte,
	) (*certifier.PoetCert, error)
}

// certifierService is used to certify nodeID for registering in the poet.
// It holds the certificates and can recertify if needed.
type certifierService interface {
	// Certificate acquires a certificate for the ID in the given certifier.
	// The certificate confirms that the ID is verified and it can be later used to submit in poet.
	Certificate(
		ctx context.Context,
		id types.NodeID,
		certifierAddress *url.URL,
		pubkey []byte,
	) (*certifier.PoetCert, error)

	DeleteCertificate(id types.NodeID, pubkey []byte) error
}

type poetDbAPI interface {
	Proof(types.PoetProofRef) (*types.PoetProof, *types.Hash32, error)
	ProofForRound(poetID []byte, roundID string) (*types.PoetProof, error)
	ValidateAndStore(ctx context.Context, proofMessage *types.PoetProofMessage) error
}

var (
	ErrPostClientClosed       = errors.New("post client closed")
	ErrPostClientNotConnected = errors.New("post service not registered")
)

type AtxBuilder interface {
	Register(sig *signing.EdSigner)
}

type postService interface {
	Client(nodeId types.NodeID) (PostClient, error)
}

type PostClient interface {
	Info(ctx context.Context) (*types.PostInfo, error)
	Proof(ctx context.Context, challenge []byte) (*types.Post, *types.PostInfo, error)
}

type PostStates interface {
	Set(id types.NodeID, state types.PostState)
	Get() map[types.NodeID]types.PostState
}
