package service

import (
	"context"
	"io"
	"os/exec"
)

// CommandRunner abstracts external command execution so services can be
// tested without invoking real binaries.
type CommandRunner interface {
	// Output runs the command in dir and returns its stdout.
	Output(ctx context.Context, dir, name string, args ...string) ([]byte, error)
	// Stream runs the command in dir, writing stdout and stderr to the
	// given writers as they are produced.
	Stream(ctx context.Context, dir string, stdout, stderr io.Writer, name string, args ...string) error
}

type execRunner struct{}

// NewExecRunner returns a CommandRunner backed by os/exec.
func NewExecRunner() CommandRunner {
	return execRunner{}
}

func (execRunner) Output(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}

func (execRunner) Stream(ctx context.Context, dir string, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// lineWriter buffers written bytes and invokes emit for each complete line.
type lineWriter struct {
	buf  []byte
	emit func(line string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := -1
		for i, b := range w.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		if line != "" {
			w.emit(line)
		}
	}
	return len(p), nil
}

// Flush emits any buffered partial line.
func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}
