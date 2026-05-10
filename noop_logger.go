package bf

// noopLogger is a Logger implementation that discards all messages.
// It is used as the default logger when none is supplied via WithLogger,
// keeping the library free of mandatory logging dependencies.
type noopLogger struct{}

func (noopLogger) Debug(...any)          {}
func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Info(...any)           {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warn(...any)           {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Error(...any)          {}
func (noopLogger) Errorf(string, ...any) {}
