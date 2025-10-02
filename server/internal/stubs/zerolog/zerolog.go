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
}

// newEvent creates a new event for the given level.
func (l Logger) newEvent(level Level) *Event {
	return &Event{logger: l, level: level}
}

// Info starts an info-level event.
func (l Logger) Info() *Event { return l.newEvent(InfoLevel) }

// Error starts an error-level event.
func (l Logger) Error() *Event { return l.newEvent(InfoLevel) }

// Fatal starts a fatal-level event.
func (l Logger) Fatal() *Event { return l.newEvent(InfoLevel) }

// Err attaches an error to the event for compatibility.
func (e *Event) Err(err error) *Event {
	e.err = err
	return e
}

// Msg writes the message to the underlying writer when available.
func (e *Event) Msg(msg string) {
	if e.logger.writer != nil {
		if e.err != nil {
			_, _ = fmt.Fprintf(e.logger.writer, "%s: %v\n", msg, e.err)
			return
		}
		_, _ = fmt.Fprintln(e.logger.writer, msg)
	}
}

// Msgf writes a formatted message to the underlying writer.
func (e *Event) Msgf(format string, args ...any) {
	if e.logger.writer != nil {
		if e.err != nil {
			args = append(args, e.err)
		}
		_, _ = fmt.Fprintf(e.logger.writer, format+"\n", args...)
	}
}
