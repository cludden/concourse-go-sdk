package sdk

import (
	"context"
	"io"
	"os"
)

type contextKey int

const (
	stderrKey contextKey = iota
)

// ContextWithStdErr returns a child context with the resource's configured
// stderr writer
func ContextWithStdErr(ctx context.Context, stderr io.Writer) context.Context {
	return context.WithValue(ctx, stderrKey, stderr)
}

// StdErrFromContext extracts the resource's configured stderr writer from the
// given context value
func StdErrFromContext(ctx context.Context) io.Writer {
	if stderr, ok := ctx.Value(stderrKey).(io.Writer); ok && stderr != nil {
		return stderr
	}
	return os.Stderr
}
