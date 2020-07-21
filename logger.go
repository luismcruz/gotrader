package gotrader

// Logger is an interface of a logger behaviour used by gotrader,
// with it the user will be able to configure logging level and message format
// at free will. The idea is to have a generic interface already implemented by a
// by some logging packages, without importing them directly.
type Logger interface {
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
}
