package flog

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var l = newAppLogger()

func newAppLogger() *zap.Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		os.Stdout,
		zap.DebugLevel,
	)
	logger := zap.New(core)
	defer func() { _ = logger.Sync() }()

	return logger
}

func Debug(msg string, fields ...interface{}) {
	kvs := zapFields(fields)
	l.Debug(msg, kvs...)
}

func Info(msg string, fields ...interface{}) {
	kvs := zapFields(fields)
	l.Info(msg, kvs...)
}

func Infof(format string, a ...interface{}) {
	l.Info(fmt.Sprintf(format, a...))
}

func Warn(msg string, fields ...interface{}) {
	kvs := zapFields(fields)
	l.Warn(msg, kvs...)
}

func Warnf(format string, a ...interface{}) {
	l.Warn(fmt.Sprintf(format, a...))
}

func Error(err error, fields ...interface{}) {
	kvs := zapFields(fields)
	l.Error(err.Error(), kvs...)
}

func Errorf(format string, a ...interface{}) {
	l.Error(fmt.Sprintf(format, a...))
}

func Panic(err error, fields ...interface{}) {
	kvs := zapFields(fields)
	l.Panic(err.Error(), kvs...)
}

func Panicf(format string, a ...interface{}) {
	l.Panic(fmt.Sprintf(format, a...))
}

func Fatal(err error, fields ...interface{}) {
	kvs := zapFields(fields)
	l.Fatal(err.Error(), kvs...)
}

func Fatalf(format string, a ...interface{}) {
	l.Fatal(fmt.Sprintf(format, a...))
}

func Field(key string, value interface{}) interface{} {
	return zap.Any(key, value)
}

func zapFields(fields []interface{}) []zap.Field {
	var res []zap.Field
	for _, i := range fields {
		if f, ok := i.(zap.Field); ok {
			res = append(res, f)
		}
	}
	return res
}
