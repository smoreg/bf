package main

import (
	"fmt"
	"log/slog"
)

// slogAdapter bridges *slog.Logger to bf.Logger.
type slogAdapter struct{ l *slog.Logger }

func (a slogAdapter) Debug(args ...any)                 { a.l.Debug(fmt.Sprint(args...)) }
func (a slogAdapter) Debugf(format string, args ...any) { a.l.Debug(fmt.Sprintf(format, args...)) }
func (a slogAdapter) Info(args ...any)                  { a.l.Info(fmt.Sprint(args...)) }
func (a slogAdapter) Infof(format string, args ...any)  { a.l.Info(fmt.Sprintf(format, args...)) }
func (a slogAdapter) Warn(args ...any)                  { a.l.Warn(fmt.Sprint(args...)) }
func (a slogAdapter) Warnf(format string, args ...any)  { a.l.Warn(fmt.Sprintf(format, args...)) }
func (a slogAdapter) Error(args ...any)                 { a.l.Error(fmt.Sprint(args...)) }
func (a slogAdapter) Errorf(format string, args ...any) { a.l.Error(fmt.Sprintf(format, args...)) }
