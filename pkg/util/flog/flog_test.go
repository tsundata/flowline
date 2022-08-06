package flog

import (
	"testing"
)

func TestLogger(t *testing.T) {
	Error(xerrors.New("test error"), Field("t", t.Name()))
	Debug("debug", Field("t", t.Name()))
	Info("info", Field("t", t.Name()))
	Warn("warn", Field("t", t.Name()))
}
