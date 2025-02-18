package testcontext

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/spacemeshos/go-spacemesh/systest/parameters"
)

const (
	ParamLayersPerEpoch = "layers-per-epoch"
	ParamLayerDuration  = "layer-duration"
)

var (
	configname = flag.String(
		"configname",
		"",
		"config map name. if not empty parameters will be loaded from specified configmap",
	)
	clusters = flag.Int(
		"clusters",
		1,
		"controls tests parallelization by creating multiple spacemesh clusters at the same time",
	)
	logLevel    = zap.LevelFlag("level", zap.InfoLevel, "verbosity of the logger")
	testTimeout = flag.Duration("test-timeout", 60*time.Minute, "timeout for a single test")

	tokens     chan struct{}
	initTokens sync.Once

	failed   = make(chan struct{})
	failOnce sync.Once
)

var (
	testid = parameters.String(
		"testid", "Name of the pod that runs tests.", "",
	)
	imageFlag = parameters.String(
		"image",
		"go-spacemesh image",
		"", // repo doesn't have a `latest` tag we can default to
	)
	oldImageFlag = parameters.String(
		"old-image",
		"old go-spacemesh image to test compatibility against",
		"spacemeshos/go-spacemesh-dev:v1.7.4", // repo doesn't have a `latest` tag we can default to
	)
	bsImage = parameters.String(
		"bs-image",
		"bootstrapper image",
		"", // repo doesn't have a `latest` tag we can default to
	)
	certifierImage = parameters.String(
		"certifier-image",
		"certifier service image",
		"spacemeshos/certifier-service:latest",
	)
	poetImage = parameters.String(
		"poet-image",
		"poet server image",
		"spacemeshos/poet:latest",
	)
	postServiceImage = parameters.String(
		"post-service-image",
		"post service image",
		"spacemeshos/post-service:latest",
	)
	postInitImage = parameters.String(
		"post-init-image",
		"post init image",
		"spacemeshos/postcli:latest",
	)
	namespaceFlag = parameters.String(
		"namespace",
		"namespace for the cluster. if empty every test will use random namespace",
		"",
	)
	bootstrapDuration = parameters.Duration(
		"bootstrap-duration",
		"bootstrap time is added to genesis time. "+
			"it may take longer in cloud environments due to additional resource management",
		30*time.Second,
	)
	clusterSize = parameters.Int(
		"cluster-size",
		"size of the cluster. all test must use at most this number of smeshers",
		10,
	)
	poetSize = parameters.Int(
		"poet-size", "size of the poet servers", 2,
	)
	bsSize = parameters.Int(
		"bs-size", "size of bootstrappers", 1,
	)
	storage = parameters.String(
		"storage", "<class>=<size> for the storage", "standard=1Gi",
	)
	keep = parameters.Bool(
		"keep", "if true cluster will not be removed after test is finished",
	)
	nodeSelector = parameters.NewParameter(
		"node-selector", "select where test pods will be scheduled",
		stringToString{},
		func(value string) (stringToString, error) {
			rst := stringToString{}
			if err := rst.Set(value); err != nil {
				return nil, err
			}
			return rst, nil
		},
	)
	LayerDuration = parameters.Duration(
		ParamLayerDuration, "layer duration in seconds", 5*time.Second,
	)
	LayersPerEpoch = parameters.Int(
		ParamLayersPerEpoch, "number of layers in an epoch", 4,
	)
)

const nsfile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

const (
	keepLabel     = "keep"
	poetSizeLabel = "poet-size"
)

func rngName() string {
	const choices = "qwertyuiopasdfghjklzxcvbnm"
	buf := make([]byte, 4)
	for i := range buf {
		buf[i] = choices[rand.IntN(len(choices))]
	}
	return string(buf)
}

// Context must be created for every test that needs isolated cluster.
type Context struct {
	context.Context
	Client            *kubernetes.Clientset
	Parameters        *parameters.Parameters
	BootstrapDuration time.Duration
	ClusterSize       int
	BootnodeSize      int
	RemoteSize        int
	PoetSize          int
	OldSize           int
	BootstrapperSize  int
	Generic           client.Client
	TestID            string
	Keep              bool
	Namespace         string
	Image             string
	OldImage          string
	BootstrapperImage string
	CertifierImage    string
	PoetImage         string
	PostServiceImage  string
	PostInitImage     string
	Storage           struct {
		Size  string
		Class string
	}
	NodeSelector map[string]string
	Log          *zap.SugaredLogger
}

func cleanup(tb testing.TB, f func()) {
	tb.Cleanup(f)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		f()
		os.Exit(1)
	}()
}

func deleteNamespace(ctx *Context) error {
	err := ctx.Client.CoreV1().Namespaces().Delete(ctx, ctx.Namespace, apimetav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete namespace %s: %w", ctx.Namespace, err)
	}
	return nil
}

func deployNamespace(ctx *Context) error {
	_, err := ctx.Client.CoreV1().Namespaces().Apply(ctx,
		corev1.Namespace(ctx.Namespace).WithLabels(map[string]string{
			"testid":      ctx.TestID,
			keepLabel:     strconv.FormatBool(ctx.Keep),
			poetSizeLabel: strconv.Itoa(ctx.PoetSize),
		}),
		apimetav1.ApplyOptions{FieldManager: "test"})
	if err != nil {
		return fmt.Errorf("create namespace %s: %w", ctx.Namespace, err)
	}
	return nil
}

func getStorage(p *parameters.Parameters) (string, string, error) {
	value := storage.Get(p)
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed storage %s. see default value for example", value)
	}
	return parts[0], parts[1], nil
}

func updateContext(ctx *Context) error {
	ns, err := ctx.Client.CoreV1().Namespaces().Get(ctx, ctx.Namespace,
		apimetav1.GetOptions{})
	if err != nil || ns == nil {
		if k8serr.IsNotFound(err) {
			return nil
		}
		return err
	}
	keepval, exists := ns.Labels[keepLabel]
	if !exists {
		ctx.Log.Panic("invalid state. keep label should exist")
	}
	keep, err := strconv.ParseBool(keepval)
	if err != nil {
		ctx.Log.Panicw("invalid state. keep label should be parsable as a boolean",
			"keepval", keepval,
		)
	}
	ctx.Keep = ctx.Keep || keep

	psizeval := ns.Labels[poetSizeLabel]
	if err != nil {
		ctx.Log.Panic("invalid state. poet size label should exist")
	}
	psize, err := strconv.Atoi(psizeval)
	if err != nil {
		ctx.Log.Panicw("invalid state. poet size label should be parsable as an integer",
			"psizeval", psizeval,
		)
	}
	ctx.PoetSize = psize
	return nil
}

// SkipClusterLimits will not block if there are no available tokens.
func SkipClusterLimits() Opt {
	return func(c *cfg) {
		c.skipLimits = true
	}
}

// Opt is for configuring Context.
type Opt func(*cfg)

func newCfg() *cfg {
	return &cfg{}
}

type cfg struct {
	skipLimits bool
}

// New creates context for the test.
func New(t *testing.T, opts ...Opt) *Context {
	initTokens.Do(func() {
		tokens = make(chan struct{}, *clusters)
	})

	c := newCfg()
	for _, opt := range opts {
		opt(c)
	}
	if !c.skipLimits {
		tokens <- struct{}{}
		t.Cleanup(func() { <-tokens })
	}

	t.Cleanup(func() {
		if t.Failed() {
			failOnce.Do(func() { close(failed) })
		}
	})
	config, err := rest.InClusterConfig()

	// The default rate limiter is too slow 5qps and 10 burst, This will prevent the client from being throttled
	// Change the limits to the same of kubectl and argo
	// That's were those number come from
	// https://github.com/kubernetes/kubernetes/pull/105520
	// https://github.com/argoproj/argo-workflows/pull/11603/files
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(50, 300)
	require.NoError(t, err)

	clientSet, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	require.NoError(t, chaos.AddToScheme(scheme))

	// prevent sigs.k8s.io/controller-runtime from complaining about log.SetLogger never being called
	log.SetLogger(k8szap.New())
	generic, err := client.New(config, client.Options{Scheme: scheme})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), *testTimeout)
	t.Cleanup(cancel)

	podns, err := os.ReadFile(nsfile)
	require.NoError(t, err, "reading nsfile at %s", nsfile)
	paramsData, err := clientSet.CoreV1().ConfigMaps(string(podns)).Get(ctx, *configname, apimetav1.GetOptions{})
	require.NoError(t, err, "get cfgmap %s/%s", string(podns), *configname)

	p := parameters.FromValues(paramsData.Data)

	class, size, err := getStorage(p)
	require.NoError(t, err)

	ns := namespaceFlag.Get(p)
	if len(ns) == 0 {
		ns = "test-" + rngName()
	}
	clSize := clusterSize.Get(p)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(
		zap.IncreaseLevel(logLevel),
		zap.AddCaller(),
		zap.AddStacktrace(zap.FatalLevel),
	))
	cctx := &Context{
		Context:           ctx,
		Parameters:        p,
		Namespace:         ns,
		BootstrapDuration: bootstrapDuration.Get(p),
		Client:            clientSet,
		Generic:           generic,
		TestID:            testid.Get(p),
		Keep:              keep.Get(p),
		ClusterSize:       clSize,
		BootnodeSize:      max(2, (clSize/1000)*2),
		RemoteSize:        clSize / 2, // 50% of smeshers are remote
		PoetSize:          poetSize.Get(p),
		OldSize:           clSize / 4, // 25% of smeshers are old (use previous version of go-spacemesh)
		BootstrapperSize:  bsSize.Get(p),
		Image:             imageFlag.Get(p),
		OldImage:          oldImageFlag.Get(p),
		BootstrapperImage: bsImage.Get(p),
		CertifierImage:    certifierImage.Get(p),
		PoetImage:         poetImage.Get(p),
		PostServiceImage:  postServiceImage.Get(p),
		PostInitImage:     postInitImage.Get(p),
		NodeSelector:      nodeSelector.Get(p),
		Log:               logger.Sugar().Named(t.Name()),
	}
	cctx.Storage.Class = class
	cctx.Storage.Size = size
	err = updateContext(cctx)
	require.NoError(t, err)
	if !cctx.Keep {
		cleanup(t, func() {
			if err := deleteNamespace(cctx); err != nil {
				cctx.Log.Errorw("cleanup failed", "error", err)
				return
			}
			cctx.Log.Infow("namespace was deleted", "namespace", cctx.Namespace)
		})
	}
	require.NoError(t, deployNamespace(cctx))
	cctx.Log.Infow("using", "namespace", cctx.Namespace)
	return cctx
}

func (c *Context) CheckFail() error {
	select {
	case <-failed:
		return errors.New("test suite failed. aborting test execution")
	default:
	}
	return nil
}
