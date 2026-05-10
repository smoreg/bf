package bf

import "testing"

func TestNoopLogger_AllMethodsAreSafe(t *testing.T) {
	var l Logger = noopLogger{}
	l.Debug("x")
	l.Debugf("%s", "x")
	l.Info("x")
	l.Infof("%s", "x")
	l.Warn("x")
	l.Warnf("%s", "x")
	l.Error("x")
	l.Errorf("%s", "x")
}
