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

type writerConfig struct {
	fileName    string
	syncOnWrite bool
	customIO    io.Writer
}

// WithFileName sets a fixed file name rather than daily rotation (e.g., "metrics.jsonl").
func WithFileName(name string) WriterOption {
	return func(c *writerConfig) {
		c.fileName = name
	}
}

// WithSyncOnWrite enables flushing and syncing to disk on every write.
func WithSyncOnWrite(sync bool) WriterOption {
	return func(c *writerConfig) {
		c.syncOnWrite = sync
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
	mu          sync.Mutex
	dataDir     string
	repoID      string
	dirPath     string
	fixedFile   string
	syncOnWrite bool
	customIO    io.Writer

	currentFile *os.File
	currentDate string
	closed      bool
}

// NewWriter creates a new JSONL metrics writer for the specified repository.
func NewWriter(dataDir, repoID string, opts ...WriterOption) (*Writer, error) {
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
func (w *Writer) WriteAny(v any) error {
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

	filePath := filepath.Join(w.dirPath, targetName)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("opening metrics file %s: %w", filePath, err)
	}

	w.currentFile = f
	w.currentDate = dateStr
	return f, nil
}

// Sync flushes buffered data to underlying storage.
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
