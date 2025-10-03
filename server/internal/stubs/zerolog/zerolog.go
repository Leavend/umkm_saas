package zerolog

import (
	"fmt"
	"io"
)

// Level represents logging severity.
type Level int8

const (
	// DebugLevel is the most verbose level.
	DebugLevel Level = iota
	// InfoLevel is the default informational level.
	InfoLevel
)

// Logger is a lightweight stub implementing the methods consumed in the code base.
type Logger struct {
	level  Level
	writer io.Writer
}

// New constructs a new stub logger writing to the provided writer.
func New(w io.Writer) Logger {
	return Logger{writer: w}
}

// Level clones the logger with the desired severity level.
func (l Logger) Level(level Level) Logger {
	l.level = level
	return l
}

// With returns a context builder preserving the logger state.
func (l Logger) With() Context {
	return Context{logger: l}
}

// Context helps build derived loggers.
type Context struct {
	logger Logger
}

// Timestamp is a no-op that keeps API compatibility.
func (c Context) Timestamp() Context {
	return c
}

// Logger finalizes the context chain and returns the underlying logger.
func (c Context) Logger() Logger {
	return c.logger
}

// Output clones the logger with a different writer.
func (l Logger) Output(w io.Writer) Logger {
	l.writer = w
	return l
}

// ConsoleWriter emulates the zerolog console writer configuration.
type ConsoleWriter struct {
	Out        io.Writer
	TimeFormat string
}

// Write satisfies the io.Writer interface for compatibility with logger.Output.
func (c ConsoleWriter) Write(p []byte) (int, error) {
	if c.Out != nil {
		return c.Out.Write(p)
	}
	return len(p), nil
}

// Event represents a structured log event.
type Event struct {
	logger Logger
	level  Level
	err    error
	fields []field
}

type field struct {
	key   string
	value string
}

// newEvent creates a new event for the given level.
func (l Logger) newEvent(level Level) *Event {
	return &Event{logger: l, level: level}
}

// Info starts an info-level event.
func (l Logger) Info() *Event { return l.newEvent(InfoLevel) }

// Warn starts a warn-level event.
func (l Logger) Warn() *Event { return l.newEvent(InfoLevel) }

// Error starts an error-level event.
func (l Logger) Error() *Event { return l.newEvent(InfoLevel) }

// Fatal starts a fatal-level event.
func (l Logger) Fatal() *Event { return l.newEvent(InfoLevel) }

// Str records a string field on the event for compatibility with zerolog.
func (e *Event) Str(key, value string) *Event {
	e.fields = append(e.fields, field{key: key, value: value})
	return e
}

// Err attaches an error to the event for compatibility.
func (e *Event) Err(err error) *Event {
	e.err = err
	return e
}

// Msg writes the message to the underlying writer when available.
func (e *Event) Msg(msg string) {
	if e.logger.writer != nil {
		line := msg
		if len(e.fields) > 0 {
			line = fmt.Sprintf("%s %s", formatFields(e.fields), msg)
		}
		if e.err != nil {
			line = fmt.Sprintf("%s: %v", line, e.err)
		}
		_, _ = fmt.Fprintln(e.logger.writer, line)
	}
}

// Msgf writes a formatted message to the underlying writer.
func (e *Event) Msgf(format string, args ...any) {
	if e.logger.writer != nil {
		prefix := ""
		if len(e.fields) > 0 {
			prefix = formatFields(e.fields) + " "
		}
		if e.err != nil {
			args = append(args, e.err)
		}
		_, _ = fmt.Fprintf(e.logger.writer, prefix+format+"\n", args...)
	}
}

// formatFields formats structured fields for the stub logger output.
func formatFields(fields []field) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, fmt.Sprintf("%s=%s", f.key, f.value))
	}
	return "{" + join(parts, ", ") + "}"
}

// join is a minimal replacement for strings.Join to avoid pulling additional deps.
func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
