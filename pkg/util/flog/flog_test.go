package flog

import (
	"errors"
	"testing"
)

func TestLogger(t *testing.T) {
	Error(errors.New("test error"), Field("t", t.Name()))
	Debug("debug", Field("t", t.Name()))
	Info("info", Field("t", t.Name()))
	Warn("warn", Field("t", t.Name()))
}
