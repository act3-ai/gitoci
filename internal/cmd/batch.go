// Package cmd implements utilities for interpreting and responding to commands sent by Git.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
)

// BatchReadWriter supports both reading from and writing to Git in batches.
type BatchReadWriter interface {
	BatchReader
	BatchWriter
}

// BatchReader extends Reader to support reading sets of commands
// provided by Git.
type BatchReader interface {
	Reader

	// ReadBatch reads lines from Git until an empty line is encountered.
	ReadBatch(context.Context) ([]Git, error)
}

type BatchWriter interface {
	Writer

	// WriteBatch writes a batch of messages to Git, which
	// MAY need to be followed up with a flush.
	WriteBatch(...string) error
}

// type ReadWriter interface {
// 	Reader
// 	Writer
// }

// Reader reads a single command from Git.
type Reader interface {
	// Read reads a single line from Git.
	Read(context.Context) (Git, error)
}

// Writer is used to Write single lines to Git, completed with a Flush.
type Writer interface {
	// Write buffers a single line write to Git. One or more
	// calls MAY need to be followed up with a flush.
	Write(string) error

	// Flush writes buffered Write(s) to Git, optionally followed up with a blank line.
	Flush(bool) error
}

// batcher implements BatchReadWriter.
type batcher struct {
	in  *bufio.Scanner
	out *bufio.Writer
}

// NewBatcher returns a buffered BatchReadWriter.
func NewBatcher(in io.Reader, out io.Writer) BatchReadWriter {
	return &batcher{
		in:  bufio.NewScanner(in),
		out: bufio.NewWriter(out),
	}
}

// Read parses a single command received by Git.
func (b *batcher) Read(ctx context.Context) (Git, error) {
	ok := b.in.Scan()
	switch {
	case !ok && b.in.Err() != nil:
		return Git{}, fmt.Errorf("reading single command from Git: %w", b.in.Err())
	case !ok:
		// EOF
		return Git{Cmd: Empty}, nil
	default:
		txt := b.in.Text()
		slog.DebugContext(ctx, "read line from Git", "text", txt)
		cmd, err := Parse(ctx, txt)
		if err != nil {
			return Git{}, fmt.Errorf("parsing Git command: %w", err)
		}
		return cmd, nil
	}
}

// ReadBatch reads lines from Git until an empty line is encountered.
func (b *batcher) ReadBatch(ctx context.Context) ([]Git, error) {
	result := make([]Git, 0, 2)
	for b.in.Scan() {
		line := b.in.Text()
		if line == "" {
			break
		}
		cmd, err := Parse(ctx, line)
		if err != nil {
			return nil, fmt.Errorf("parsing Git command: %w", err)
		}
		result = append(result, cmd)
	}
	if b.in.Err() != nil {
		return result, fmt.Errorf("scanning input: %w", b.in.Err())
	}
	return result, nil
}

// WriteBatch writes Message(s) to Git, completing the batch with a blank line, and flushing the buffered writes to Git.
func (b *batcher) WriteBatch(lines ...string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(b.out, line); err != nil {
			return fmt.Errorf("writing to Git, line: %s: %w", line, err)
		}
	}

	return b.Flush(true)
}

// Write buffers a single line write to Git, must be followed up with a flush.
func (b *batcher) Write(line string) error {
	if _, err := fmt.Fprintln(b.out, line); err != nil {
		return fmt.Errorf("writing to Git, line: %s: %w", line, err)
	}

	return nil
}

// Flush writes buffered Write(s) to Git, followed up with a blank line.
func (b *batcher) Flush(blankLine bool) error {
	if blankLine {
		if _, err := fmt.Fprintln(b.out); err != nil {
			return fmt.Errorf("writing blank line to Git: %w", err)
		}
	}

	if err := b.out.Flush(); err != nil {
		return fmt.Errorf("flushing writes to Git: %w", err)
	}

	return nil
}
