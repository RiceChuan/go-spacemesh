package logtest

import (
	"os"
	"testing"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/spacemeshos/go-spacemesh/log"
)

const testLogLevel = "TEST_LOG_LEVEL"

// New creates log.Log instance that will use testing.TB.Log internally.
func New(tb testing.TB, override ...zapcore.Level) log.Log {
	var level zapcore.Level
	if len(override) > 0 {
		level = override[0]
	} else {
		lvl := os.Getenv(testLogLevel)
		if len(lvl) == 0 {
			return log.NewNop()
		}
		if err := level.Set(lvl); err != nil {
			panic(err)
		}
	}
	return log.NewFromLog(zaptest.NewLogger(tb, zaptest.Level(level))).Named(tb.Name())
}
