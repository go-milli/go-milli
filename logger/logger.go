// Package logger defines a logging abstraction and helpers.
package logger

// DefaultLogger is the package default logger.
var DefaultLogger Logger = NewLogger()

// DefaultHelper is the package default helper wrapping DefaultLogger.
var DefaultHelper *Helper = NewHelper(DefaultLogger)

// Logger is a generic logging interface (micro-style).
type Logger interface {
	// Init initializes options
	Init(options ...Option) error
	// Options returns current options
	Options() Options
	// Fields sets fields to always be logged
	Fields(fields map[string]interface{}) Logger
	// Log writes a log entry
	Log(level Level, v ...interface{})
	// Logf writes a formatted log entry
	Logf(level Level, format string, v ...interface{})
	// String returns the name of logger
	String() string
}

func Init(opts ...Option) error {
	return DefaultLogger.Init(opts...)
}

func Fields(fields map[string]interface{}) Logger {
	return DefaultLogger.Fields(fields)
}

func Log(level Level, v ...interface{}) {
	DefaultLogger.Log(level, v...)
}

func Logf(level Level, format string, v ...interface{}) {
	DefaultLogger.Logf(level, format, v...)
}

func String() string {
	return DefaultLogger.String()
}

// LoggerOrDefault returns the provided logger or the DefaultLogger if nil.
func LoggerOrDefault(l Logger) Logger {
	if l == nil {
		return DefaultLogger
	}
	return l
}
