package log

import (
	"errors"
	"testing"
)

func TestLogger(t *testing.T) {
	FLog.Error(errors.New("test error"), FLog.Field("t", t.Name()))
	FLog.Debug("debug", FLog.Field("t", t.Name()))
	FLog.Info("info", FLog.Field("t", t.Name()))
	FLog.Warn("warn", FLog.Field("t", t.Name()))
}
