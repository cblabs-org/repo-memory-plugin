package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Iterator provides iteration over op batches.
type Iterator struct {
	reader       *Reader
	ctx          context.Context
	afterBatchID string
	beforeTime   *time.Time
	pullNext     func() (*OpBatch, error, bool)
	pullStop     func()
}

// NewIterator creates a new Iterator.
func NewIterator(r *Reader, ctx context.Context) *Iterator {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Iterator{
		reader: r,
		ctx:    ctx,
	}
}

// Seq returns the iter.Seq2 sequence for this iterator.
func (it *Iterator) Seq() iter.Seq2[*OpBatch, error] {
	return it.reader.iterateInternal(it.ctx, it.afterBatchID, it.beforeTime)
}

// Next pulls the next batch from the iterator. Returns io.EOF when iteration is done.
func (it *Iterator) Next() (*OpBatch, error) {
	if it.pullNext == nil {
		next, stop := iter.Pull2(it.Seq())
		it.pullNext = next
		it.pullStop = stop
	}
	batch, err, ok := it.pullNext()
	if !ok {
		return nil, io.EOF
	}
	return batch, err
}

// Close terminates any active pull iteration and frees associated resources.
func (it *Iterator) Close() {
	if it.pullStop != nil {
		it.pullStop()
		it.pullNext = nil
		it.pullStop = nil
	}
}

// Iterate yields batches in ULID order (which is chronological).
// It walks month directories in order, then files within each month in ULID order.
func (r *Reader) Iterate(ctx context.Context) iter.Seq2[*OpBatch, error] {
	if ctx == nil {
		ctx = context.Background()
	}
	it := &Iterator{reader: r, ctx: ctx}
	return it.Seq()
}

// IterateAfter yields only batches strictly after the given batch ID.
func (r *Reader) IterateAfter(ctx context.Context, afterBatchID string) iter.Seq2[*OpBatch, error] {
	if ctx == nil {
		ctx = context.Background()
	}
	it := &Iterator{reader: r, ctx: ctx, afterBatchID: afterBatchID}
	return it.Seq()
}

// IterateBefore yields only batches whose CreatedAt is before the given commit's timestamp.
// The beforeCommit is a git commit reference; its timestamp is resolved via git log -1 --format=%cI <commit>.
func (r *Reader) IterateBefore(ctx context.Context, beforeCommit string) iter.Seq2[*OpBatch, error] {
	if ctx == nil {
		ctx = context.Background()
	}
	return func(yield func(*OpBatch, error) bool) {
		cutoff, err := r.resolveCommitTime(ctx, beforeCommit)
		if err != nil {
			yield(nil, err)
			return
		}
		for batch, bErr := range r.IterateBeforeTime(ctx, cutoff) {
			if !yield(batch, bErr) {
				return
			}
		}
	}
}

// IterateBeforeTime yields only batches whose CreatedAt is before the specified cutoff timestamp.
func (r *Reader) IterateBeforeTime(ctx context.Context, cutoff time.Time) iter.Seq2[*OpBatch, error] {
	if ctx == nil {
		ctx = context.Background()
	}
	it := &Iterator{reader: r, ctx: ctx, beforeTime: &cutoff}
	return it.Seq()
}

func (r *Reader) iterateInternal(ctx context.Context, afterBatchID string, beforeTime *time.Time) iter.Seq2[*OpBatch, error] {
	return func(yield func(*OpBatch, error) bool) {
		if ctx == nil {
			ctx = context.Background()
		}

		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}

		entries, err := os.ReadDir(r.baseDir)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err != nil {
			yield(nil, fmt.Errorf("read ops base dir %s: %w", r.baseDir, err))
			return
		}

		var monthDirs []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if _, parseErr := time.Parse("2006-01", name); parseErr == nil {
				monthDirs = append(monthDirs, name)
			}
		}
		sort.Strings(monthDirs)

		seenAfter := (afterBatchID == "")

		for _, monthDir := range monthDirs {
			if err := ctx.Err(); err != nil {
				yield(nil, err)
				return
			}

			monthPath := filepath.Join(r.baseDir, monthDir)
			fileEntries, err := os.ReadDir(monthPath)
			if err != nil {
				if !yield(nil, fmt.Errorf("read month dir %s: %w", monthPath, err)) {
					return
				}
				continue
			}

			var jsonFiles []os.DirEntry
			for _, f := range fileEntries {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
					jsonFiles = append(jsonFiles, f)
				}
			}

			sort.Slice(jsonFiles, func(i, j int) bool {
				return jsonFiles[i].Name() < jsonFiles[j].Name()
			})

			for _, f := range jsonFiles {
				if err := ctx.Err(); err != nil {
					yield(nil, err)
					return
				}

				filePath := filepath.Join(monthPath, f.Name())
				batch, err := r.ReadBatch(filePath)
				if err != nil {
					if !yield(nil, fmt.Errorf("read batch %s: %w", filePath, err)) {
						return
					}
					continue
				}

				if !seenAfter {
					if batch.BatchID == afterBatchID {
						seenAfter = true
						continue
					}
					if strings.ToUpper(batch.BatchID) > strings.ToUpper(afterBatchID) {
						seenAfter = true
					} else {
						continue
					}
				}

				if beforeTime != nil && !batch.CreatedAt.UTC().Before(beforeTime.UTC()) {
					continue
				}

				if !yield(batch, nil) {
					return
				}
			}
		}
	}
}

func (r *Reader) resolveCommitTime(ctx context.Context, beforeCommit string) (time.Time, error) {
	if strings.TrimSpace(beforeCommit) == "" {
		return time.Time{}, errors.New("beforeCommit cannot be empty")
	}

	dir := r.gitDir
	if dir == "" {
		dir = r.baseDir
	}

	var cmd *exec.Cmd
	if dir != "" {
		cmd = exec.CommandContext(ctx, "git", "-C", dir, "log", "-1", "--format=%cI", beforeCommit)
	} else {
		cmd = exec.CommandContext(ctx, "git", "log", "-1", "--format=%cI", beforeCommit)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		if dir != "." && dir != "" {
			cmd2 := exec.CommandContext(ctx, "git", "log", "-1", "--format=%cI", beforeCommit)
			if out2, err2 := cmd2.CombinedOutput(); err2 == nil {
				out = out2
				err = nil
			}
		}
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("git log commit %q: %w (output: %s)", beforeCommit, err, strings.TrimSpace(string(out)))
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return time.Time{}, fmt.Errorf("no commit timestamp found for %q", beforeCommit)
	}

	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse commit timestamp %q: %w", raw, err)
	}

	return t, nil
}
