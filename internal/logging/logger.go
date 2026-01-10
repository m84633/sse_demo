package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func New() (*zap.Logger, error) {
	if os.Getenv("GIN_MODE") == "release" {
		if err := os.MkdirAll("logs", 0o755); err != nil {
			return nil, err
		}
		encoderCfg := zap.NewProductionEncoderConfig()
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(os.Stdout),
				zapcore.AddSync(&lumberjack.Logger{
					Filename:   "logs/app.log",
					MaxSize:    50,
					MaxBackups: 5,
					MaxAge:     14,
					Compress:   true,
				}),
			),
			zap.InfoLevel,
		)
		return zap.New(core), nil
	}
	return zap.NewDevelopment()
}
