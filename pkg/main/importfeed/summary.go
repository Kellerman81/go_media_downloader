package importfeed

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// ImportSummary aggregates per-entry results of a list import run so callers
// can log one line per run instead of trawling per-entry log output.
// Safe for concurrent use - feed imports run in worker pools.
type ImportSummary struct {
	Imported atomic.Int64
	Ignored  atomic.Int64
	Failed   atomic.Int64
}

type importSummaryCtxKey struct{}

// WithImportSummary attaches a fresh ImportSummary to the context. The
// JobImport* entry points record their per-entry outcome into it when one is
// attached, so callers opt in without any function signature changes.
func WithImportSummary(ctx context.Context) (context.Context, *ImportSummary) {
	s := &ImportSummary{}
	return context.WithValue(ctx, importSummaryCtxKey{}, s), s
}

// summaryFromContext returns the attached ImportSummary, or nil.
func summaryFromContext(ctx context.Context) *ImportSummary {
	s, _ := ctx.Value(importSummaryCtxKey{}).(*ImportSummary)
	return s
}

// recordImportResult classifies one import outcome into the context's summary
// when one is attached. ignoredErrs are sentinels that count as "ignored"
// rather than failed (e.g. errIgnoredMovie, errJobRunning).
func recordImportResult(ctx context.Context, err error, ignoredErrs ...error) {
	s := summaryFromContext(ctx)
	if s == nil {
		return
	}

	if err == nil {
		s.Imported.Add(1)
		return
	}

	for i := range ignoredErrs {
		if errors.Is(err, ignoredErrs[i]) {
			s.Ignored.Add(1)
			return
		}
	}

	s.Failed.Add(1)
}

// Log emits one aggregated info line for the run when anything was processed.
func (s *ImportSummary) Log(cfgName, listName string) {
	imported, ignored, failed := s.Imported.Load(), s.Ignored.Load(), s.Failed.Load()
	if imported == 0 && ignored == 0 && failed == 0 {
		return
	}

	logger.Logtype("info", 2).
		Str(logger.StrConfig, cfgName).
		Str(logger.StrListname, listName).
		Int64("imported", imported).
		Int64("ignored", ignored).
		Int64("failed", failed).
		Msg("List import summary")
}
