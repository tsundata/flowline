package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var FLog = newAppLogger()

type appLogger struct {
	logger *zap.Logger
}

func newAppLogger() *appLogger {
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

	return &appLogger{logger: logger}
}

func (l *appLogger) Debug(msg string, fields ...interface{}) {
	kvs := zapFields(fields)
	l.logger.Debug(msg, kvs...)
}

func (l *appLogger) Info(msg string, fields ...interface{}) {
	kvs := zapFields(fields)
	l.logger.Info(msg, kvs...)
}

func (l *appLogger) Warn(msg string, fields ...interface{}) {
	kvs := zapFields(fields)
	l.logger.Warn(msg, kvs...)
}

func (l *appLogger) Error(err error, fields ...interface{}) {
	kvs := zapFields(fields)
	l.logger.Error(err.Error(), kvs...)
}

func (l *appLogger) Panic(err error, fields ...interface{}) {
	kvs := zapFields(fields)
	l.logger.Panic(err.Error(), kvs...)
}

func (l *appLogger) Fatal(err error, fields ...interface{}) {
	kvs := zapFields(fields)
	l.logger.Fatal(err.Error(), kvs...)
}

func (l *appLogger) Field(key string, value interface{}) interface{} {
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
