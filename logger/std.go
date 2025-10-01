package logger

import (
	"fmt"
	"log"
	"os"
)

type stdLogger struct {
	opts   Options
	fields map[string]interface{}
}

// NewLogger builds a new std logger based on options.
func NewLogger(opts ...Option) Logger {
	options := Options{Level: InfoLevel, Fields: map[string]interface{}{}, Out: os.Stderr, CallerSkipCount: 2}
	l := &stdLogger{opts: options, fields: map[string]interface{}{}}
	_ = l.Init(opts...)
	return l
}

func (l *stdLogger) Init(opts ...Option) error {
	for _, o := range opts {
		o(&l.opts)
	}
	return nil
}

func (l *stdLogger) Options() Options { return l.opts }

func (l *stdLogger) Fields(fields map[string]interface{}) Logger {
	nf := make(map[string]interface{}, len(l.fields)+len(fields))
	for k, v := range l.fields {
		nf[k] = v
	}
	for k, v := range fields {
		nf[k] = v
	}
	return &stdLogger{opts: l.opts, fields: nf}
}

func (l *stdLogger) Log(level Level, v ...interface{}) {
	if !l.opts.Level.Enabled(level) {
		return
	}
	log.Printf("%s %s %s", level.String(), localGetMessage("", v), formatMap(l.fields))
}

func (l *stdLogger) Logf(level Level, format string, v ...interface{}) {
	if !l.opts.Level.Enabled(level) {
		return
	}
	log.Printf("%s %s %s", level.String(), fmt.Sprintf(format, v...), formatMap(l.fields))
}

func (l *stdLogger) String() string { return "std" }

func formatMap(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	return "{" + join(pairs, ", ") + "}"
}

func join(ss []string, sep string) string {
	switch len(ss) {
	case 0:
		return ""
	case 1:
		return ss[0]
	}
	n := len(sep) * (len(ss) - 1)
	for i := 0; i < len(ss); i++ {
		n += len(ss[i])
	}
	b := make([]byte, 0, n)
	for i := 0; i < len(ss); i++ {
		if i > 0 {
			b = append(b, sep...)
		}
		b = append(b, ss[i]...)
	}
	return string(b)
}

func localGetMessage(template string, fmtArgs []interface{}) string {
	if len(fmtArgs) == 0 {
		return template
	}
	if template != "" {
		return fmt.Sprintf(template, fmtArgs...)
	}
	if len(fmtArgs) == 1 {
		if str, ok := fmtArgs[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(fmtArgs...)
}
