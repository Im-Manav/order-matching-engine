package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func Init(env string) error {
	var cfg zap.Config

	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var err error
	Log, err = cfg.Build()
	if err != nil {
		return err
	}
	return nil
}

func Info(msg string, fields ...zap.Field)  { Log.Info(msg, fields...) }
func Error(msg string, fields ...zap.Field) { Log.Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { Log.Fatal(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { Log.Warn(msg, fields...) }
func Sync()                                 { _ = Log.Sync() }

func InitForTest() {
	Log, _ = zap.NewDevelopment()
}

// Convenience: wrap a standard error as a zap field
func Err(err error) zap.Field {
	return zap.Error(err)
}

func String(key, val string) zap.Field  { return zap.String(key, val) }
func Int(key string, val int) zap.Field { return zap.Int(key, val) }
func Any(key string, val any) zap.Field { return zap.Any(key, val) }
