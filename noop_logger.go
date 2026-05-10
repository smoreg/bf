package bf

// noopLogger is a Logger implementation that discards all messages.
// It is used as the default logger when none is supplied via WithLogger,
// keeping the library free of mandatory logging dependencies.
type noopLogger struct{}

func (noopLogger) Debug(args ...any)                 { _ = args }
func (noopLogger) Debugf(format string, args ...any) { _, _ = format, args }
func (noopLogger) Info(args ...any)                  { _ = args }
func (noopLogger) Infof(format string, args ...any)  { _, _ = format, args }
func (noopLogger) Warn(args ...any)                  { _ = args }
func (noopLogger) Warnf(format string, args ...any)  { _, _ = format, args }
func (noopLogger) Error(args ...any)                 { _ = args }
func (noopLogger) Errorf(format string, args ...any) { _, _ = format, args }
