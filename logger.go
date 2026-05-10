package bf

// Logger is the logging contract used by the bot. Any logger implementing
// these methods (logrus, zap, slog wrapper, etc.) can be plugged via WithLogger.
type Logger interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
}
