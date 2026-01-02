package config

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggerConfig struct {
	Level string
	Env   string
}

func NewLogger(cfg LoggerConfig) (*zap.Logger, error) {
	var zapCfg zap.Config

	if cfg.Env == "prod" {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
	}

	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)

	return zapCfg.Build()
}
