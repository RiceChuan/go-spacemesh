package sql

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

var (
	// ErrClosed is returned if database is closed.
	ErrClosed = errors.New("database closed")
	// ErrNoConnection is returned if pooled connection is not available.
	ErrNoConnection = errors.New("database: no free connection")
	// ErrNotFound is returned if requested record is not found.
	ErrNotFound = errors.New("database: not found")
	// ErrObjectExists is returned if database constraints didn't allow to insert an object.
	ErrObjectExists = errors.New("database: object exists")
	// ErrConflict is returned if database constraints didn't allow to update an object.
	ErrConflict = errors.New("database: conflict")
	// ErrTooNew is returned if database version is newer than expected.
	ErrTooNew = errors.New("database version is too new")
	// ErrOldSchema is returned when the database version differs from the expected one
	// and migrations are disabled.
	ErrOldSchema = errors.New("old database version")
)

const (
	beginDefault   = "BEGIN;"
	beginImmediate = "BEGIN IMMEDIATE;"
)

// Statement is an sqlite statement.
type Statement = sqlite.Stmt

// Encoder for parameters.
// Both positional parameters:
// select block from blocks where id = ?1;
//
// and named parameters are supported:
// select blocks from blocks where id = @id;
//
// For complete information see https://www.sqlite.org/c3ref/bind_blob.html.
type Encoder func(*Statement)

// Decoder for sqlite rows.
type Decoder func(*Statement) bool

func defaultConf() *conf {
	return &conf{
		enableMigrations:           true,
		connections:                16,
		logger:                     zap.NewNop(),
		schema:                     &Schema{},
		checkSchemaDrift:           true,
		handleIncompleteMigrations: true,
	}
}

type conf struct {
	uri                        string
	enableMigrations           bool
	forceFresh                 bool
	forceMigrations            bool
	connections                int
	vacuumState                int
	enableLatency              bool
	cache                      bool
	cacheSizes                 map[QueryCacheKind]int
	logger                     *zap.Logger
	schema                     *Schema
	allowSchemaDrift           bool
	checkSchemaDrift           bool
	temp                       bool
	handleIncompleteMigrations bool
	exclusive                  bool
	readOnly                   bool
}

// WithConnections overwrites number of pooled connections.
func WithConnections(n int) Opt {
	return func(c *conf) {
		c.connections = n
	}
}

// WithLogger specifies logger for the database.
func WithLogger(logger *zap.Logger) Opt {
	return func(c *conf) {
		c.logger = logger
	}
}

// WithMigrationsDisabled disables migrations for the database.
// The migrations are enabled by default.
func WithMigrationsDisabled() Opt {
	return func(c *conf) {
		c.enableMigrations = false
	}
}

// WithVacuumState will execute vacuum if database version before the migration was less or equal to the provided value.
func WithVacuumState(i int) Opt {
	return func(c *conf) {
		c.vacuumState = i
	}
}

// WithLatencyMetering enables metric that track latency for every database query.
// Note that it will be a significant amount of data, and should not be enabled on
// multiple nodes by default.
func WithLatencyMetering(enable bool) Opt {
	return func(c *conf) {
		c.enableLatency = enable
	}
}

// WithQueryCache enables in-memory caching of results of some queries.
func WithQueryCache(enable bool) Opt {
	return func(c *conf) {
		c.cache = enable
	}
}

// WithQueryCacheSizes sets query cache sizes for the specified cache kinds.
func WithQueryCacheSizes(sizes map[QueryCacheKind]int) Opt {
	return func(c *conf) {
		if c.cacheSizes == nil {
			c.cacheSizes = maps.Clone(sizes)
		} else {
			maps.Copy(c.cacheSizes, sizes)
		}
	}
}

// WithForceMigrations forces database to run all the migrations instead
// of using a schema snapshot in case of a fresh database.
func WithForceMigrations(force bool) Opt {
	return func(c *conf) {
		c.forceMigrations = true
	}
}

// WithSchema specifies database schema script.
func WithDatabaseSchema(schema *Schema) Opt {
	return func(c *conf) {
		c.schema = schema
	}
}

// WithAllowSchemaDrift prevents Open from failing upon schema drift when schema drift
// checks are enabled. A warning is printed instead.
func WithAllowSchemaDrift(allow bool) Opt {
	return func(c *conf) {
		c.allowSchemaDrift = allow
	}
}

// WithNoCheckSchemaDrift disables schema drift checks.
func WithNoCheckSchemaDrift() Opt {
	return func(c *conf) {
		c.checkSchemaDrift = false
	}
}

func withForceFresh() Opt {
	return func(c *conf) {
		c.forceFresh = true
	}
}

// WithTemp specifies temporary database mode.
// For the temporary database, the migrations are always run in place, and vacuuming is
// nover done.  PRAGMA journal_mode=OFF and PRAGMA synchronous=OFF are used.
func WithTemp() Opt {
	return func(c *conf) {
		c.temp = true
	}
}

func withDisableIncompleteMigrationHandling() Opt {
	return func(c *conf) {
		c.handleIncompleteMigrations = false
	}
}

// WithExclusive specifies that the database is to be open in exclusive mode.
// This means that no other processes can open the database at the same time.
// If the database is already open by any process, this Open will fail.
// Any subsequent attempts by other processes to open the database will fail until this db
// handle is closed.
// In Exclusive mode, the database supports just one concurrent connection.
func WithExclusive() Opt {
	return func(c *conf) {
		c.exclusive = true
	}
}

// WithReadOnly specifies that the database is to be open in read-only mode.
func WithReadOnly() Opt {
	return func(c *conf) {
		c.readOnly = true
	}
}

// Opt for configuring database.
type Opt func(c *conf)

// OpenInMemory creates an in-memory database.
func OpenInMemory(opts ...Opt) (*sqliteDatabase, error) {
	opts = append(opts, withForceFresh())
	// Unique uri is needed to avoid sharing the same in-memory database,
	// while allowing multiple connections to the same database.
	uri := fmt.Sprintf("file:mem-%d-%d?mode=memory&cache=shared",
		rand.Uint64(), rand.Uint64())
	return Open(uri, opts...)
}

// InMemory creates an in-memory database for testing and panics if
// there's an error.
func InMemory(opts ...Opt) *sqliteDatabase {
	db, err := OpenInMemory(opts...)
	if err != nil {
		panic(err)
	}
	return db
}

// InMemoryTest returns an in-mem database for testing and ensures database is closed during `tb.Cleanup`.
func InMemoryTest(tb testing.TB, opts ...Opt) *sqliteDatabase {
	// When using empty DB schema, we don't want to check for schema drift due to
	// "PRAGMA user_version = 0;" in the initial schema retrieved from the DB.
	db := InMemory(append(opts, WithNoCheckSchemaDrift())...)
	tb.Cleanup(func() { db.Close() })
	return db
}

// Open database with options.
//
// Database is opened in WAL mode and pragma synchronous=normal.
// https://sqlite.org/wal.html
// https://www.sqlite.org/pragma.html#pragma_synchronous
func Open(uri string, opts ...Opt) (*sqliteDatabase, error) {
	config := defaultConf()
	config.uri = uri
	for _, opt := range opts {
		opt(config)
	}
	if !config.temp && config.handleIncompleteMigrations && !config.forceFresh {
		if err := handleIncompleteCopyMigration(config); err != nil {
			return nil, err
		}
	}
	return openDB(config)
}

func openDB(config *conf) (db *sqliteDatabase, err error) {
	logger := config.logger.With(zap.String("uri", config.uri))
	var flags sqlite.OpenFlags
	if !config.forceFresh {
		flags = sqlite.SQLITE_OPEN_URI |
			sqlite.SQLITE_OPEN_NOMUTEX
		if !config.temp {
			// Note that SQLITE_OPEN_WAL is not handled by SQLITE api itself,
			// but rather by the crawshaw library which executes
			// PRAGMA journal_mode=WAL in this case.
			// We don't want it for temporary databases as they're not
			// using any journal
			flags |= sqlite.SQLITE_OPEN_WAL
		}
		if !config.readOnly {
			flags |= sqlite.SQLITE_OPEN_READWRITE
		} else {
			flags |= sqlite.SQLITE_OPEN_READONLY
		}
	}

	freshDB := config.forceFresh
	if config.exclusive {
		config.connections = 1
	}
	pool, err := sqlitex.Open(config.uri, flags, config.connections)
	if err != nil {
		if config.forceFresh || sqlite.ErrCode(err) != sqlite.SQLITE_CANTOPEN {
			return nil, fmt.Errorf("open db %s: %w", config.uri, err)
		}
		flags |= sqlite.SQLITE_OPEN_CREATE
		freshDB = true
		pool, err = sqlitex.Open(config.uri, flags, config.connections)
		if err != nil {
			return nil, fmt.Errorf("create db %s: %w", config.uri, err)
		}
	}
	db = &sqliteDatabase{pool: pool}
	defer func() {
		// If something goes wrong, close the database even in case of a
		// panic. This is important for tests that verify incomplete migration.
		if r := recover(); r != nil {
			db.Close()
			panic(r)
		}
	}()
	// In case of VACUUM INTO based migration, prepareDB may close this database and
	// open another one.
	actualDB, err := prepareDB(logger, db, config, freshDB)
	if err != nil {
		db.Close()
		return nil, err
	}
	return actualDB, nil
}

func prepareDB(logger *zap.Logger, db *sqliteDatabase, config *conf, freshDB bool) (*sqliteDatabase, error) {
	var err error

	if config.enableLatency {
		db.latency = newQueryLatency()
	}

	if config.temp {
		// Temporary database is used for migration and is deleted if migrations
		// fail, so we make it faster by disabling journaling and synchronous
		// writes.
		if _, err := db.Exec("PRAGMA journal_mode=OFF", nil, nil); err != nil {
			return nil, fmt.Errorf("PRAGMA journal_mode=OFF: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=OFF", nil, nil); err != nil {
			return nil, fmt.Errorf("PRAGMA synchronous=OFF: %w", err)
		}
	}

	if config.exclusive {
		if err := db.startExclusive(); err != nil {
			return nil, fmt.Errorf("start exclusive: %w", err)
		}
	}

	if freshDB && !config.forceMigrations {
		if err := config.schema.Apply(db); err != nil {
			return nil, fmt.Errorf("error running schema script: %w", err)
		}
	} else if db, err = ensureDBSchemaUpToDate(logger, db, config); err != nil {
		// ensureDBSchemaUpToDate may replace the original database and open the new one,
		// in which case the original db is already closed but we must close the new one.
		// If there are migrations to be done in place without vacuuming,
		// the original db is returned and we must close it if there's an error.
		if db != nil {
			db.Close()
		}
		return nil, err
	}

	if config.checkSchemaDrift {
		loaded, err := LoadDBSchemaScript(db)
		if err != nil {
			return nil, fmt.Errorf("error loading database schema: %w", err)
		}
		diff := config.schema.Diff(loaded)
		switch {
		case diff == "": // ok
		case config.allowSchemaDrift:
			logger.Warn("database schema drift detected",
				zap.String("uri", config.uri),
				zap.String("diff", diff),
			)
		default:
			return nil, fmt.Errorf("schema drift detected (uri %s):\n%s", config.uri, diff)
		}
	}

	if config.cache {
		logger.Debug("using query cache", zap.Any("sizes", config.cacheSizes))
		db.queryCache = &queryCache{cacheSizesByKind: config.cacheSizes}
	}
	db.queryCount.Store(0)
	return db, nil
}

func ensureDBSchemaUpToDate(logger *zap.Logger, db *sqliteDatabase, config *conf) (*sqliteDatabase, error) {
	before, after, err := config.schema.CheckDBVersion(logger, db)
	switch {
	case err != nil:
		return db, fmt.Errorf("check db version: %w", err)
	case before == after:
		return db, nil
	case before > after:
		return db, fmt.Errorf("%w: %d > %d", ErrTooNew, before, after)
	case !config.enableMigrations:
		return db, fmt.Errorf("%w: %d < %d", ErrOldSchema, before, after)
	case config.temp:
		// Temporary database, do migrations without transactions
		// and sync afterwards
		return db, config.schema.MigrateTempDB(logger, db, before)
	case config.vacuumState != 0 &&
		before <= config.vacuumState &&
		strings.HasPrefix(config.uri, "file:"):
		logger.Info("running migrations",
			zap.Int("current version", before),
			zap.Int("target version", after),
		)
		return db.copyMigrateDB(config)
	}

	logger.Info("running migrations in-place",
		zap.Int("current version", before),
		zap.Int("target version", after),
	)
	return db, config.schema.Migrate(logger, db, before, config.vacuumState)
}

func Version(uri string) (int, error) {
	pool, err := sqlitex.Open(uri, sqlite.SQLITE_OPEN_READONLY, 1)
	if err != nil {
		return 0, fmt.Errorf("open db %s: %w", uri, err)
	}
	db := &sqliteDatabase{pool: pool}
	v, err := version(db)
	if err != nil {
		db.Close()
		return 0, err
	}
	if err := db.Close(); err != nil {
		return 0, fmt.Errorf("close db %s: %w", uri, err)
	}
	return v, nil
}

// deleteDB deletes the database at the specified path by removing /path/to/DB* files.
// If the database doesn't exist, no error is returned.
// In addition to what DROP DATABASE does, this also removes the migration marker file.
func deleteDB(path string) error {
	// https://www.sqlite.org/tempfiles.html plus marker *_done
	for _, suffix := range []string{"", "-journal", "-wal", "-shm", "_done"} {
		file := path + suffix
		if err := os.Remove(file); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("remove %s: %w", file, err)
		}
	}
	return nil
}

// moveMigratedDB runs "VACUUM INTO" on the database at fromPath and
// replaces the database at toPath with the vacuumed one. The database
// at fromPath is deleted after the operation.
func moveMigratedDB(config *conf, fromPath, toPath string) (err error) {
	config.logger.Warn("finalizing migration by moving the temporary DB to the original path",
		zap.String("fromPath", fromPath),
		zap.String("toPath", toPath))
	// Try to open the temporary migrated DB in exclusive mode before deleting the
	// original one.
	// If the temporary DB is being copied to the original path by another
	// process, this will fail and the original database will not be deleted.
	// We don't use the proper database schema here because the temporary DB
	// may have been created with a different set of migrations.
	db, err := Open("file:"+fromPath,
		WithLogger(config.logger),
		WithConnections(1),
		WithTemp(),
		WithNoCheckSchemaDrift(),
		WithExclusive(),
	)
	if err != nil {
		return fmt.Errorf("open temporary DB %s: %w", fromPath, err)
	}
	if err := deleteDB(toPath); err != nil {
		return err
	}
	if err := db.vacuumInto(toPath); err != nil {
		db.Close()
		return err
	}
	// Open the freshly vacuumed DB in exclusive mode to avoid race condition when
	// another process also tries to vacuum the temporary DB into the original path
	// after we close the temporary DB.
	origDB, err := Open("file:"+toPath,
		WithLogger(config.logger),
		WithConnections(1),
		WithMigrationsDisabled(),
		WithNoCheckSchemaDrift(),
		withDisableIncompleteMigrationHandling(),
		WithExclusive(),
	)
	if err != nil {
		return fmt.Errorf("open vacuumed DB %s: %w", toPath, err)
	}
	if err := db.Close(); err != nil {
		origDB.Close()
		return fmt.Errorf("close temporary DB %s: %w", fromPath, err)
	}
	if err := deleteDB(fromPath); err != nil {
		origDB.Close()
		return err
	}
	if err := origDB.Close(); err != nil {
		return fmt.Errorf("close DB %s after migration: %w", toPath, err)
	}
	return nil
}

func dbMigrationPaths(uri string) (dbPath, migratedPath string, err error) {
	url, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parse uri: %w", err)
	}
	if url.Scheme != "file" {
		return "", "", nil
	}
	path := url.Opaque
	if path == "" {
		path = url.Path
	}
	return path, path + "_migrate", nil
}

// handleIncompleteCopyMigration handles incomplete copy-based migrations.
// It only works for 'file:' URIs, doing nothing for other URIs.
// It first checks if there's a copy of the database with "_migrate" suffix.
// If it's there, it checks if the migration is complete by checking if
// DBNAME_migrate_done file exists. It it doesn't, the migration is considered
// incomplete and the migrated database is removed. If DBNAME_migrate_done
// file exists, the migration is finalized by running "VACUUM INTO" on the
// migrated database and replacing the original, after which the migrated
// database is deleted.
func handleIncompleteCopyMigration(config *conf) error {
	dbPath, migratedPath, err := dbMigrationPaths(config.uri)
	if err != nil {
		return fmt.Errorf("getting DB migration paths: %w", err)
	}
	if migratedPath == "" {
		return nil
	}
	if _, err := os.Stat(migratedPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// no migration in progress
			return nil
		}
		return fmt.Errorf("stat %s: %w", migratedPath, err)
	}
	if _, err := os.Stat(migratedPath + "_done"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// incomplete migration, delete the temporary DB to start over
			// after that
			config.logger.Warn("incomplete migration detected, deleting the temporary DB",
				zap.String("path", migratedPath))
			return deleteDB(migratedPath)
		}
	}

	// the migration is complete except for the last step
	return moveMigratedDB(config, migratedPath, dbPath)
}

// Interceptor is invoked on every query after it's added to a database using
// PushIntercept. The query will fail if Interceptor returns an error.
type Interceptor func(query string) error

// Database represents a database.
type Database interface {
	Executor
	QueryCache
	// Close closes the database.
	Close() error
	// QueryCount returns the number of queries executed on the database.
	QueryCount() int
	// QueryCache returns the query cache for this database, if it's present,
	// or nil otherwise.
	QueryCache() QueryCache
	// Tx creates deferred sqlite transaction.
	//
	// Deferred transactions are not started until the first statement.
	// Transaction may be started in read mode and automatically upgraded to write mode
	// after one of the write statements.
	//
	// https://www.sqlite.org/lang_transaction.html
	Tx(ctx context.Context) (Transaction, error)
	// WithTx starts a new transaction and passes it to the exec function.
	// It then commits the transaction if the exec function doesn't return an error,
	// and rolls it back otherwise.
	// If the context is canceled, the currently running SQL statement is interrupted.
	WithTx(ctx context.Context, exec func(Transaction) error) error
	// TxImmediate begins a new immediate transaction on the database, that is,
	// a transaction that starts a write immediately without waiting for a write
	// statement.
	// The transaction returned from this function must always be released by calling
	// its Release method. Release rolls back the transaction if it hasn't been
	// committed.
	// If the context is canceled, the currently running SQL statement is interrupted.
	TxImmediate(ctx context.Context) (Transaction, error)
	// WithTxImmediate starts a new immediate transaction and passes it to the exec
	// function.
	// An immediate transaction is started immediately, without waiting for a write
	// statement.
	// It then commits the transaction if the exec function doesn't return an error,
	// and rolls it back otherwise.
	// If the context is canceled, the currently running SQL statement is interrupted.
	WithTxImmediate(ctx context.Context, exec func(Transaction) error) error
	// WithConnection executes the provided function with a connection from the
	// database pool.
	// If many queries are to be executed in a row, but there's no need for an
	// explicit transaction which may be long-running and thus block
	// WAL checkpointing, it may be preferable to use a single connection for
	// it to avoid database pool overhead.
	// The connection is released back to the pool after the function returns.
	// If the context is canceled, the currently running SQL statement is interrupted.
	WithConnection(ctx context.Context, exec func(Executor) error) error
	// Intercept adds an interceptor function to the database. The interceptor
	// functions are invoked upon each query on the database, including queries
	// executed within transactions.
	// The query will fail if the interceptor returns an error.
	// The interceptor can later be removed using RemoveInterceptor with the same key.
	Intercept(key string, fn Interceptor)
	// RemoveInterceptor removes the interceptor function with specified key from the database.
	RemoveInterceptor(key string)
}

// Transaction represents a transaction.
type Transaction interface {
	Executor
	// Commit commits the transaction.
	Commit() error
	// Release releases the transaction. If the transaction hasn't been committed,
	// it's rolled back.
	Release() error
}

type sqliteDatabase struct {
	*queryCache
	pool *sqlitex.Pool

	closed   bool
	closeMux sync.Mutex

	latency    *prometheus.HistogramVec
	queryCount atomic.Int64

	interceptMtx sync.Mutex
	interceptors map[string]Interceptor
}

var _ Database = &sqliteDatabase{}

func (db *sqliteDatabase) getConn(ctx context.Context) *sqlite.Conn {
	start := time.Now()
	conn := db.pool.Get(ctx)
	if conn != nil {
		connWaitLatency.Observe(time.Since(start).Seconds())
	}
	return conn
}

func (db *sqliteDatabase) getTx(ctx context.Context, initstmt string) (*sqliteTx, error) {
	if db.closed {
		return nil, ErrClosed
	}
	conCtx, cancel := context.WithCancel(ctx)
	conn := db.getConn(conCtx)
	if conn == nil {
		cancel()
		return nil, ErrNoConnection
	}
	tx := &sqliteTx{queryCache: db.queryCache, db: db, conn: conn, freeConn: cancel}
	if err := tx.begin(initstmt); err != nil {
		cancel()
		db.pool.Put(conn)
		return nil, err
	}
	return tx, nil
}

func (db *sqliteDatabase) withTx(ctx context.Context, initstmt string, exec func(Transaction) error) (err error) {
	tx, err := db.getTx(ctx, initstmt)
	if err != nil {
		return err
	}
	defer func() {
		if rErr := tx.Release(); rErr != nil && err == nil {
			err = fmt.Errorf("release tx: %w", rErr)
		}
	}()
	if err := exec(tx); err != nil {
		tx.queryCache.ClearCache()
		return err
	}
	return tx.Commit()
}

func (db *sqliteDatabase) startExclusive() error {
	conn := db.getConn(context.Background())
	if conn == nil {
		return ErrNoConnection
	}
	defer db.pool.Put(conn)
	// We don't need to wait for long if the database is busy
	conn.SetBusyTimeout(1 * time.Millisecond)
	// From SQLite docs:
	// When the locking-mode is set to EXCLUSIVE, the database connection
	// never releases file-locks. The first time the database is read in
	// EXCLUSIVE mode, a shared lock is obtained and held. The first time the
	// database is written, an exclusive lock is obtained and held.
	if _, err := exec(conn, "PRAGMA locking_mode=EXCLUSIVE", nil, nil); err != nil {
		return fmt.Errorf("PRAGMA locking_mode=EXCLUSIVE: %w", err)
	}
	// We need to perform a transaction to have the database actually locked.
	// From SQLite docs, regarding BEGIN EXCLUSIVE / BEGIN IMMEDIATE:
	// EXCLUSIVE is similar to IMMEDIATE in that a write transaction is
	// started immediately. EXCLUSIVE and IMMEDIATE are the same in WAL mode,
	// but in other journaling modes, EXCLUSIVE prevents other database
	// connections from reading the database while the transaction is
	// underway.
	_, err := exec(conn, "BEGIN EXCLUSIVE", nil, nil)
	if err != nil {
		return fmt.Errorf("error starting the EXCLUSIVE transaction: %w", err)
	}
	if _, err := exec(conn, "COMMIT", nil, nil); err != nil {
		return fmt.Errorf("error committing the EXCLUSIVE transaction: %w", err)
	}
	return nil
}

// Tx implements Database.
func (db *sqliteDatabase) Tx(ctx context.Context) (Transaction, error) {
	return db.getTx(ctx, beginDefault)
}

// WithTx implements Database.
func (db *sqliteDatabase) WithTx(ctx context.Context, exec func(Transaction) error) error {
	return db.withTx(ctx, beginDefault, exec)
}

// TxImmediate implements Database.
func (db *sqliteDatabase) TxImmediate(ctx context.Context) (Transaction, error) {
	return db.getTx(ctx, beginImmediate)
}

// WithTxImmediate implements Database.
func (db *sqliteDatabase) WithTxImmediate(ctx context.Context, exec func(Transaction) error) error {
	return db.withTx(ctx, beginImmediate, exec)
}

func (db *sqliteDatabase) runInterceptors(query string) error {
	db.interceptMtx.Lock()
	defer db.interceptMtx.Unlock()
	for _, interceptFn := range db.interceptors {
		if err := interceptFn(query); err != nil {
			return err
		}
	}
	return nil
}

// Exec implements Executor.
//
// If you care about atomicity of the operation (for example writing rewards to multiple accounts)
// Tx should be used. Otherwise sqlite will not guarantee that all side-effects of operations are
// applied to the database if machine crashes.
//
// Note that Exec will block until database is closed or statement has finished.
// If application needs to control statement execution lifetime use one of the transaction.
func (db *sqliteDatabase) Exec(query string, encoder Encoder, decoder Decoder) (int, error) {
	if err := db.runInterceptors(query); err != nil {
		return 0, err
	}

	if db.closed {
		return 0, ErrClosed
	}
	db.queryCount.Add(1)
	conn := db.getConn(context.Background())
	if conn == nil {
		return 0, ErrNoConnection
	}
	defer db.pool.Put(conn)
	if db.latency != nil {
		start := time.Now()
		defer func() {
			db.latency.WithLabelValues(query).Observe(float64(time.Since(start)))
		}()
	}
	return exec(conn, query, encoder, decoder)
}

// Close implements Database.
func (db *sqliteDatabase) Close() error {
	db.closeMux.Lock()
	defer db.closeMux.Unlock()
	if db.closed {
		return nil
	}
	if err := db.pool.Close(); err != nil {
		return fmt.Errorf("close pool: %w", err)
	}
	db.closed = true
	return nil
}

// WithConnection implements Database.
func (db *sqliteDatabase) WithConnection(ctx context.Context, exec func(Executor) error) error {
	if db.closed {
		return ErrClosed
	}
	conCtx, cancel := context.WithCancel(ctx)
	conn := db.getConn(conCtx)
	defer func() {
		cancel()
		db.pool.Put(conn)
	}()
	if conn == nil {
		return ErrNoConnection
	}
	return exec(&sqliteConn{queryCache: db.queryCache, db: db, conn: conn})
}

// Intercept adds an interceptor function to the database. The interceptor functions
// are invoked upon each query. The query will fail if the interceptor returns an error.
// The interceptor can later be removed using RemoveInterceptor with the same key.
func (db *sqliteDatabase) Intercept(key string, fn Interceptor) {
	db.interceptMtx.Lock()
	defer db.interceptMtx.Unlock()
	if db.interceptors == nil {
		db.interceptors = make(map[string]Interceptor)
	}
	db.interceptors[key] = fn
}

// PopIntercept removes the interceptor function with specified key from the database.
// If there's no such interceptor, the function does nothing.
func (db *sqliteDatabase) RemoveInterceptor(key string) {
	db.interceptMtx.Lock()
	defer db.interceptMtx.Unlock()
	delete(db.interceptors, key)
}

// vacuumInto runs VACUUM INTO on the database and saves the vacuumed
// database at toPath.
func (db *sqliteDatabase) vacuumInto(toPath string) error {
	if _, err := db.Exec("VACUUM INTO ?1", func(stmt *Statement) {
		stmt.BindText(1, toPath)
	}, nil); err != nil {
		return fmt.Errorf("vacuum into %s: %w", toPath, err)
	}
	return nil
}

// copyMigrateDB performs a copy-based migration of the database.
// The source database is always closed by this function.
// Upon success, the migrated database is opened.
func (db *sqliteDatabase) copyMigrateDB(config *conf) (finalDB *sqliteDatabase, err error) {
	dbPath, migratedPath, err := dbMigrationPaths(config.uri)
	if err != nil {
		return nil, fmt.Errorf("getting DB migration paths: %w", err)
	}
	if migratedPath == "" {
		return nil, fmt.Errorf("cannot migrate database, only file DBs are supported: %s", config.uri)
	}

	// Before we start the migration, re-open the database in exclusive mode
	// so that no other connections will be able to use it.
	// This will fail if another process is already using this database.
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("error closing DB: %w", err)
	}

	excDB, err := Open("file:"+dbPath,
		WithLogger(config.logger),
		WithConnections(1),
		WithNoCheckSchemaDrift(),
		WithExclusive(),
	)
	if err != nil {
		return nil, fmt.Errorf("error opening the database in exclusive mode: %v", err)
	}
	defer excDB.Close()

	// instead of just copying the source database to the temporary migration DB, use VACUUM INTO.
	// This is somewhat slower but achieves two goals:
	// 1. The lock is held on the source database while it's being copied
	// 2. If the source database has a lot of free pages for whatever reason, those
	// are not copied, saving disk space
	config.logger.Info("making a temporary copy of the database",
		zap.String("path", dbPath),
		zap.String("target", migratedPath))
	if err := excDB.vacuumInto(migratedPath); err != nil {
		if err := deleteDB(migratedPath); err != nil {
			config.logger.Error(
				"incomplete temporary copy of the database couldn't be deleted",
				zap.String("path", migratedPath),
				zap.Error(err),
			)
		}
		return nil, err
	}

	// Opening the temporary migrated DB runs the actual migrations on it.
	// We disable vacuuming here because we're going to vacuum the temporary DB
	// into the original one.
	opts := []Opt{
		WithLogger(config.logger),
		WithConnections(1),
		WithTemp(),
		WithDatabaseSchema(config.schema),
		WithExclusive(),
	}
	if !config.checkSchemaDrift {
		opts = append(opts, WithNoCheckSchemaDrift())
	}
	migratedDB, err := Open("file:"+migratedPath, opts...)
	if err != nil {
		if err := deleteDB(migratedPath); err != nil {
			config.logger.Error(
				"incomplete temporary copy of the database couldn't be deleted",
				zap.String("path", migratedPath),
				zap.Error(err),
			)
		}
		return nil, fmt.Errorf("process temporary DB %s: %w", migratedPath, err)
	}
	defer migratedDB.Close()

	// Make sure the temporary DB is fully synced to the disk before creating the marker file.
	// We don't need wal_checkpoint(TRUNCATE) here as we're going to delete the temporary DB.
	if _, err := migratedDB.Exec("PRAGMA wal_checkpoint(FULL)", nil, nil); err != nil {
		if err := deleteDB(migratedPath); err != nil {
			config.logger.Error(
				"incomplete temporary copy of the database couldn't be deleted",
				zap.String("path", migratedPath),
				zap.Error(err),
			)
		}
		return nil, fmt.Errorf("checkpoint temporary DB %s: %w", migratedPath, err)
	}

	// Create the marker file to indicate that the migration is complete and make sure
	// the file is written to the disk before closing the database.
	// We could create a table in the temporary database instead of the marker file,
	// but as the temporary database is opened without PRAGMA journal_mode=OFF
	// and PRAGMA synchronous=OFF, it may become corrupt in case of a crash or power
	// outage, so we avoid trying to open it.
	if err := createMarkerFile(migratedPath); err != nil {
		if err := deleteDB(migratedPath); err != nil {
			config.logger.Error(
				"incomplete temporary copy of the database couldn't be deleted",
				zap.String("path", migratedPath),
				zap.Error(err),
			)
		}
		// The errors returned by createMarkerFile are already descriptive enough
		// so no need to augment them
		return nil, err
	}

	// At this point, the temporary database is complete and should not be deleted
	// until we copy it to the original database location.

	// We only close the source database at the end of the migration process
	// so that the lock is held. There's a possibility that right after we
	// close the source database, another process will see the migrated database
	// and the marker file and will try to open the migrated database. If the
	if err := excDB.Close(); err != nil {
		return nil, fmt.Errorf("close db: %w", err)
	}

	// Delete the original database. VACUUM INTO will fail if the destination
	// database exists.
	if err := deleteDB(dbPath); err != nil {
		return nil, fmt.Errorf("delete original DB %s: %w", dbPath, err)
	}

	// Overwrite the original database with the migrated one.
	// The lock is held on the temporary DB during this, preventing concurrent
	// go-spacemesh instances to attempt the same operation.
	config.logger.Info("moving the temporary DB to original location", zap.String("path", dbPath))
	if err := migratedDB.vacuumInto(dbPath); err != nil {
		return nil, err
	}

	// Open the final DB in the exclusive mode before deleting the source DB, so one of the locks
	// is always held. The migrations are already run, so we're disabling them.
	origExclusive := config.exclusive
	config.enableMigrations = false
	config.exclusive = true
	finalDB, err = openDB(config)
	if err != nil {
		return nil, fmt.Errorf("open final DB %s: %w", config.uri, err)
	}

	if err := migratedDB.Close(); err != nil {
		finalDB.Close()
		return nil, fmt.Errorf("close temporary DB %s: %w", migratedPath, err)
	}

	// Now we can delete the temporary DB and the marker file.
	if err := deleteDB(migratedPath); err != nil {
		finalDB.Close()
		return nil, err
	}

	// If we were not intending to open the database in exclusive mode,
	// reopen it in the normal mode
	if !origExclusive {
		if err := finalDB.Close(); err != nil {
			return nil, fmt.Errorf("close final DB: %w", err)
		}
		config.exclusive = false
		finalDB, err = openDB(config)
		if err != nil {
			return nil, fmt.Errorf("open final DB %s: %w", config.uri, err)
		}
	}

	return finalDB, nil
}

func createMarkerFile(basePath string) error {
	markerPath := basePath + "_done"
	f, err := os.Create(markerPath)
	if err != nil {
		return fmt.Errorf("create marker file %s: %w", markerPath, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(markerPath)
		return fmt.Errorf("sync/close marker file %s: %w", markerPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close marker file %s: %w", markerPath, err)
	}
	return nil
}

// QueryCount returns the number of queries executed, including failed
// queries, but not counting transaction start / commit / rollback.
func (db *sqliteDatabase) QueryCount() int {
	return int(db.queryCount.Load())
}

// Return database's QueryCache.
func (db *sqliteDatabase) QueryCache() QueryCache {
	return db.queryCache
}

func exec(conn *sqlite.Conn, query string, encoder Encoder, decoder Decoder) (int, error) {
	stmt, err := conn.Prepare(query)
	if err != nil {
		return 0, fmt.Errorf("prepare %s: %w", query, err)
	}
	if encoder != nil {
		encoder(stmt)
	}
	defer stmt.ClearBindings()
	defer stmt.Reset()

	rows := 0
	for {
		row, err := stmt.Step()
		if err != nil {
			return 0, fmt.Errorf("step %d: %w", rows, mapSqliteError(err))
		}
		if !row {
			return rows, nil
		}
		rows++
		// exhaust iterator
		if decoder == nil {
			continue
		}
		if !decoder(stmt) {
			if err := stmt.Reset(); err != nil {
				return rows, fmt.Errorf("statement reset %w", err)
			}
			return rows, nil
		}
	}
}

// sqliteTx is wrapper for database transaction.
type sqliteTx struct {
	*queryCache
	db        *sqliteDatabase
	conn      *sqlite.Conn
	freeConn  func()
	committed bool
	err       error
}

func (tx *sqliteTx) begin(initstmt string) error {
	stmt := tx.conn.Prep(initstmt)
	_, err := stmt.Step()
	if err != nil {
		return fmt.Errorf("begin: %w", mapSqliteError(err))
	}
	return nil
}

// Commit transaction.
func (tx *sqliteTx) Commit() error {
	stmt := tx.conn.Prep("COMMIT;")
	_, tx.err = stmt.Step()
	if tx.err != nil {
		return mapSqliteError(tx.err)
	}
	tx.committed = true
	return nil
}

// Release transaction. Every transaction that was created must be released.
func (tx *sqliteTx) Release() error {
	defer tx.db.pool.Put(tx.conn)
	if tx.committed {
		tx.freeConn()
		return nil
	}
	stmt := tx.conn.Prep("ROLLBACK")
	_, tx.err = stmt.Step()
	tx.freeConn()
	return mapSqliteError(tx.err)
}

// Exec query.
func (tx *sqliteTx) Exec(query string, encoder Encoder, decoder Decoder) (int, error) {
	if err := tx.db.runInterceptors(query); err != nil {
		return 0, fmt.Errorf("running query interceptors: %w", err)
	}

	tx.db.queryCount.Add(1)
	if tx.db.latency != nil {
		start := time.Now()
		defer func() {
			tx.db.latency.WithLabelValues(query).Observe(float64(time.Since(start)))
		}()
	}
	return exec(tx.conn, query, encoder, decoder)
}

type sqliteConn struct {
	*queryCache
	db   *sqliteDatabase
	conn *sqlite.Conn
}

func (c *sqliteConn) Exec(query string, encoder Encoder, decoder Decoder) (int, error) {
	if err := c.db.runInterceptors(query); err != nil {
		return 0, fmt.Errorf("running query interceptors: %w", err)
	}

	c.db.queryCount.Add(1)
	if c.db.latency != nil {
		start := time.Now()
		defer func() {
			c.db.latency.WithLabelValues(query).Observe(float64(time.Since(start)))
		}()
	}
	return exec(c.conn, query, encoder, decoder)
}

func mapSqliteError(err error) error {
	switch sqlite.ErrCode(err) {
	case sqlite.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite.SQLITE_CONSTRAINT_UNIQUE:
		return ErrObjectExists
	case sqlite.SQLITE_INTERRUPT:
		// TODO: we probably should check if there was indeed a context that was
		// canceled
		return context.Canceled
	default:
		return err
	}
}

// Blob represents a binary blob data. It can be reused efficiently
// across multiple data retrieval operations, minimizing reallocations
// of the underlying byte slice.
type Blob struct {
	Bytes []byte
}

// Resize the underlying byte slice to the specified size.
// The returned slice has length equal n, but it might have a larger capacity.
// Warning: it is not guaranteed to keep the old data.
func (b *Blob) Resize(n int) {
	if cap(b.Bytes) < n {
		b.Bytes = make([]byte, n)
	}
	b.Bytes = b.Bytes[:n]
}

func (b *Blob) FromColumn(stmt *Statement, col int) {
	if l := stmt.ColumnLen(col); l != 0 {
		b.Resize(l)
		stmt.ColumnBytes(col, b.Bytes)
	} else {
		b.Resize(0)
	}
}

// GetBlobSizes returns a slice containing the sizes of blobs
// corresponding to the specified ids. For non-existent ids the
// corresponding value is -1.
func GetBlobSizes(db Executor, cmd string, ids [][]byte) (sizes []int, err error) {
	if len(ids) == 0 {
		return nil, nil
	}

	cmd += fmt.Sprintf(" (?%s)", strings.Repeat(",?", len(ids)-1))
	sizes = make([]int, len(ids))
	for n := range sizes {
		sizes[n] = -1
	}
	m := make(map[string]int)
	for n, id := range ids {
		m[string(id)] = n
	}
	id := make([]byte, len(ids[0]))
	if _, err := db.Exec(cmd,
		func(stmt *Statement) {
			for n, id := range ids {
				stmt.BindBytes(n+1, id)
			}
		}, func(stmt *Statement) bool {
			stmt.ColumnBytes(0, id[:])
			n, found := m[string(id)]
			if !found {
				panic("BUG: bad ID retrieved from DB")
			}
			sizes[n] = stmt.ColumnInt(1)
			return true
		}); err != nil {
		return nil, fmt.Errorf("get blob sizes: %w", err)
	}

	return sizes, nil
}

// LoadBlob loads an encoded blob.
func LoadBlob(db Executor, cmd string, id []byte, blob *Blob) error {
	rows, err := db.Exec(cmd,
		func(stmt *Statement) {
			stmt.BindBytes(1, id)
		}, func(stmt *Statement) bool {
			blob.FromColumn(stmt, 0)
			return true
		},
	)
	if err != nil {
		return fmt.Errorf("get %v: %w", types.BytesToHash(id), err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: object %s", ErrNotFound, hex.EncodeToString(id))
	}
	return nil
}

// IsNull returns true if the specified result column is null.
func IsNull(stmt *Statement, col int) bool {
	return stmt.ColumnType(col) == sqlite.SQLITE_NULL
}

// StateDatabase is a Database used for Spacemesh state.
type StateDatabase interface {
	Database
	IsStateDatabase()
}

// LocalDatabase is a Database used for local node data.
type LocalDatabase interface {
	Database
	IsLocalDatabase()
}
