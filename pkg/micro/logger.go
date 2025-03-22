package micro

import "go.uber.org/zap"

// Logger interface defines the logging contract
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
}

// ZapLogger implements Logger interface using zap
type ZapLogger struct {
	*zap.Logger
}

func (zl *ZapLogger) With(fields ...zap.Field) Logger {
	return &ZapLogger{zl.Logger.With(fields...)}
}

// NewLogger creates a new logger instance
func NewLogger(level string) (Logger, error) {
	var logger *zap.Logger
	var err error

	switch level {
	case "debug":
		logger, err = zap.NewDevelopment(zap.AddStacktrace(zap.ErrorLevel))
	default:
		logger, err = zap.NewProduction(zap.AddStacktrace(zap.ErrorLevel))
	}

	if err != nil {
		return nil, err
	}

	return &ZapLogger{logger}, nil
}
