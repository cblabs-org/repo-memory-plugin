/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/metrics/writer.go
 * Description: Provides a thread-safe JSONL Writer for appending operational
 *               telemetry records to daily-rotated or fixed files under
 *               <data-dir>/<repo-id>/metrics.
 *
 * Exported Types / Functions:
 * - ErrClosed:         Sentinel error returned when writing to or syncing a closed Writer.
 * - WriterOption:      Functional option for configuring a Writer.
 * - WithFileName():    Configures a fixed output file name instead of daily date rotation.
 * - WithSyncOnWrite(): Configures fsync on every write operation.
 * - WithCustomWriter(): Configures writing directly to a custom io.Writer.
 * - Writer:            Thread-safe JSONL telemetry writer.
 *
 *   Lifecycle:
 *   - NewWriter():       Creates a Writer backed by <data-dir>/<repo-id>/metrics.
 *   - NewWriterFromIO(): Creates a Writer backed by an arbitrary io.Writer.
 *   - Close():           Closes any open file handle and marks the writer closed. [io.Closer]
 *
 *   Own Public API:
 *   - Write():                      Marshals and appends a Record as a JSONL line.
 *   - WriteAny():                   Marshals and appends any value as a JSONL line.
 *   - WriteCaptureHealth():         Populates defaults and writes a CaptureHealth record.
 *   - WriteHookExecution():         Populates defaults and writes a HookExecution record.
 *   - WriteTickHealth():            Populates defaults and writes a TickHealth record.
 *   - WriteBudgetExhaustion():      Populates defaults and writes a BudgetExhaustion record.
 *   - WriteTranscriptMeasurement(): Populates defaults and writes a TranscriptMeasurement record.
 *   - Sync():                       Flushes buffered file data to disk.
 *   - Directory():                  Returns the filesystem path of the metrics directory.
 */

package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

// ErrClosed is returned when writing to a closed Writer.
var ErrClosed = errors.New("metrics writer is closed")

// WriterOption configures a metrics Writer.
type WriterOption func(*writerConfig)

// writerConfig holds internal configuration options applied to a Writer at construction time.
type writerConfig struct {
	fileName    string    // Optional fixed file name overriding daily YYYY-MM-DD.jsonl rotation.
	syncOnWrite bool      // Whether to invoke File.Sync() after every write.
	customIO    io.Writer // Optional destination writer overriding filesystem storage.
}

// WithFileName sets a fixed file name rather than daily rotation (e.g., "metrics.jsonl").
func WithFileName(name string) WriterOption {
	return func(c *writerConfig) {
		c.fileName = name
	}
}

// WithSyncOnWrite enables flushing and syncing to disk on every write.
func WithSyncOnWrite(syncOnWrite bool) WriterOption {
	return func(c *writerConfig) {
		c.syncOnWrite = syncOnWrite
	}
}

// WithCustomWriter configures writing to an arbitrary io.Writer.
func WithCustomWriter(w io.Writer) WriterOption {
	return func(c *writerConfig) {
		c.customIO = w
	}
}

// Writer handles thread-safe appending of JSONL metrics records.
// In filesystem mode, records are persisted under <data-dir>/<repo-id>/metrics/*.jsonl (SPEC §10.4).
type Writer struct {
	mu          sync.Mutex // Serializes concurrent writes, file rotations, and Close calls.
	dataDir     string     // Base data directory path.
	repoID      string     // Repository slug scoping the <data-dir>/<repo-id>/metrics directory.
	dirPath     string     // Resolved metrics directory path (<data-dir>/<repo-id>/metrics).
	fixedFile   string     // Optional fixed JSONL filename; empty uses daily rotation.
	syncOnWrite bool       // If true, syncs the underlying os.File after every write.
	customIO    io.Writer  // Optional custom io.Writer destination.

	currentFile *os.File // Currently open daily or fixed JSONL file handle.
	currentDate string   // Date string (YYYY-MM-DD) corresponding to currentFile.
	closed      bool     // True once Close has been called.
}

// NewWriter creates a new JSONL metrics writer for the specified repository.
// The caller must call Close on the returned Writer when finished.
func NewWriter(dataDir, repoID string, opts ...WriterOption) (*Writer, error) {
	// --- Step 1: Validate inputs and apply functional options ---
	if dataDir == "" {
		dataDir = config.DataDir()
	}
	if repoID == "" {
		return nil, errors.New("repoID cannot be empty")
	}

	cfg := writerConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	// --- Step 2: Ensure the repository metrics directory exists when using filesystem storage ---
	metricsDir := filepath.Join(dataDir, repoID, "metrics")
	if cfg.customIO == nil {
		if err := os.MkdirAll(metricsDir, 0700); err != nil {
			return nil, fmt.Errorf("creating metrics directory %s: %w", metricsDir, err)
		}
	}

	return &Writer{
		dataDir:     dataDir,
		repoID:      repoID,
		dirPath:     metricsDir,
		fixedFile:   cfg.fileName,
		syncOnWrite: cfg.syncOnWrite,
		customIO:    cfg.customIO,
	}, nil
}

// NewWriterFromIO creates a metrics writer backed by an arbitrary io.Writer (e.g. for testing or piping).
func NewWriterFromIO(w io.Writer) *Writer {
	return &Writer{
		customIO: w,
	}
}

// Write marshals and appends a single Record as a JSONL line.
func (w *Writer) Write(record Record) error {
	return w.WriteAny(record)
}

// WriteAny marshals and appends any metric struct or map as a JSON line.
// It returns ErrClosed if the Writer has been closed.
func (w *Writer) WriteAny(v any) error {
	// --- Step 1: Acquire lock, check closed state, and marshal payload to JSONL ---
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrClosed
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling metrics record: %w", err)
	}
	data = append(data, '\n')

	// --- Step 2: Write to customIO or the active rotated file and optionally fsync ---
	if w.customIO != nil {
		_, err := w.customIO.Write(data)
		return err
	}

	dest, err := w.ensureFileLocked(time.Now().UTC())
	if err != nil {
		return err
	}

	if _, err := dest.Write(data); err != nil {
		return fmt.Errorf("writing metrics JSONL: %w", err)
	}

	if w.syncOnWrite {
		if err := dest.Sync(); err != nil {
			return fmt.Errorf("syncing metrics file: %w", err)
		}
	}

	return nil
}

// WriteCaptureHealth writes a CaptureHealth record.
func (w *Writer) WriteCaptureHealth(m CaptureHealth) error {
	if m.Type == "" {
		m.Type = MetricTypeCaptureHealth
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now().UTC()
	}
	if m.RepoID == "" {
		m.RepoID = w.repoID
	}
	return w.Write(m)
}

// WriteHookExecution writes a HookExecution record.
func (w *Writer) WriteHookExecution(m HookExecution) error {
	if m.Type == "" {
		m.Type = MetricTypeHookExecution
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now().UTC()
	}
	if m.RepoID == "" {
		m.RepoID = w.repoID
	}
	return w.Write(m)
}

// WriteTickHealth writes a TickHealth record.
func (w *Writer) WriteTickHealth(m TickHealth) error {
	if m.Type == "" {
		m.Type = MetricTypeTickHealth
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now().UTC()
	}
	if m.RepoID == "" {
		m.RepoID = w.repoID
	}
	return w.Write(m)
}

// WriteBudgetExhaustion writes a BudgetExhaustion record.
func (w *Writer) WriteBudgetExhaustion(m BudgetExhaustion) error {
	if m.Type == "" {
		m.Type = MetricTypeBudgetExhaustion
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now().UTC()
	}
	if m.RepoID == "" {
		m.RepoID = w.repoID
	}
	return w.Write(m)
}

// WriteTranscriptMeasurement writes a TranscriptMeasurement record.
func (w *Writer) WriteTranscriptMeasurement(m TranscriptMeasurement) error {
	if m.Type == "" {
		m.Type = MetricTypeTranscriptMeasurement
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now().UTC()
	}
	if m.RepoID == "" {
		m.RepoID = w.repoID
	}
	return w.Write(m)
}

// ensureFileLocked ensures the target file is open for appending, rotating per day if no fixed file name is set.
func (w *Writer) ensureFileLocked(t time.Time) (*os.File, error) {
	// --- Step 1: Determine target filename and reuse currentFile if still valid ---
	dateStr := t.Format("2006-01-02")
	targetName := w.fixedFile
	if targetName == "" {
		targetName = dateStr + ".jsonl"
	}

	if w.currentFile != nil {
		if w.fixedFile != "" || w.currentDate == dateStr {
			return w.currentFile, nil
		}
		// Close file for previous date
		_ = w.currentFile.Close()
		w.currentFile = nil
	}

	// --- Step 2: Open or create the target JSONL file in append mode ---
	filePath := filepath.Join(w.dirPath, targetName)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("opening metrics file %s: %w", filePath, err)
	}

	w.currentFile = f
	w.currentDate = dateStr
	return f, nil
}

// Sync flushes buffered data to underlying storage. It returns ErrClosed if the Writer is closed.
func (w *Writer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrClosed
	}

	if w.currentFile != nil {
		return w.currentFile.Sync()
	}
	return nil
}

// Close closes any open file descriptor and marks the writer as closed.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	if w.currentFile != nil {
		err := w.currentFile.Close()
		w.currentFile = nil
		return err
	}
	return nil
}

// Directory returns the metrics storage directory path.
func (w *Writer) Directory() string {
	return w.dirPath
}
