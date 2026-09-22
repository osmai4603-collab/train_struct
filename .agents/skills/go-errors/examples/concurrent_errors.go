package examples

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sync/errgroup"
)

// SafeCopyFile demonstrates how to merge a deferred close error with read and write errors
func SafeCopyFile(dst, src string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close src: %w", closeErr))
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close dst: %w", closeErr))
		}
	}()

	if _, copyErr := io.Copy(out, in); copyErr != nil {
		return fmt.Errorf("copy data: %w", copyErr)
	}

	return nil
}

// ProcessItemsConcurrently demonstrates running concurrent tasks and capturing their errors with errgroup
func ProcessItemsConcurrently(ctx context.Context, items []string) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, item := range items {
		item := item
		g.Go(func() error {
			if item == "invalid" {
				return fmt.Errorf("processing item %q failed", item)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		})
	}

	return g.Wait()
}
