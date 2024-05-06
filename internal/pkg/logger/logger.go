package logger

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const traceIDKey = "traceID"

type Config struct {
	Name  string `env-required:"true" yaml:"name"`
	Debug bool   `yaml:"debug"`
}

func NewLogger(config *Config) (*zap.SugaredLogger, error) {
	var zapConfig zap.Config

	if config.Debug {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	zapConfig.EncoderConfig.CallerKey = ""

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, errors.Wrap(err, "can't build logger")
	}

	logger = logger.With(zap.String("service", config.Name))

	return logger.Sugar(), nil
}

func TraceID(traceID string) zap.Field {
	return zap.Field{Key: traceIDKey, Type: zapcore.StringType, String: traceID}
}

func Error(err error) zap.Field {
	return zap.Error(err)
}
