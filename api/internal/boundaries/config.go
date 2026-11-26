package boundaries

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"vigilafrica/api/internal/models"
)

// Config holds the loader's operational settings (spec §2). Every field has a
// safe default (developers-go.md §2.3).
type Config struct {
	// RetryAfter: an entry that failed is not downloaded again before this has
	// passed (BOUNDARY_RETRY_HOURS, default 24).
	RetryAfter time.Duration
	// MaxArchiveBytes caps the download and the extracted member
	// (BOUNDARY_MAX_ARCHIVE_BYTES, default 128 MiB; South Africa's whole zip is
	// 93,548,039 bytes).
	MaxArchiveBytes int64
	// TxTimeout bounds each country's load transaction
	// (BOUNDARY_TX_TIMEOUT, Go duration, default 2m; must be under the lease TTL).
	TxTimeout time.Duration
	// CountryPause is the pause between attempted countries
	// (BOUNDARY_COUNTRY_PAUSE_MS, default 1000).
	CountryPause time.Duration
	// DownloadTimeout bounds one archive download. Not an env var: 3 minutes,
	// under the 5-minute lease TTL, and the lease is renewed again before the
	// transaction starts.
	DownloadTimeout time.Duration
}

// Defaults.
const (
	DefaultRetryAfter      = 24 * time.Hour
	DefaultMaxArchiveBytes = 128 << 20
	DefaultTxTimeout       = 2 * time.Minute
	DefaultCountryPause    = time.Second
	DefaultDownloadTimeout = 3 * time.Minute
)

// DefaultConfig returns the defaults above.
func DefaultConfig() Config {
	return Config{
		RetryAfter:      DefaultRetryAfter,
		MaxArchiveBytes: DefaultMaxArchiveBytes,
		TxTimeout:       DefaultTxTimeout,
		CountryPause:    DefaultCountryPause,
		DownloadTimeout: DefaultDownloadTimeout,
	}
}

// ConfigFromEnv reads BOUNDARY_LOADER_ENABLED (default true) and the settings
// above through getenv. Like the server's other operational variables, an
// invalid value is logged and replaced by its default rather than stopping
// the server. leaseTTL is the scheduler lease's TTL; BOUNDARY_TX_TIMEOUT must
// be below it.
func ConfigFromEnv(getenv func(string) string, leaseTTL time.Duration, log *slog.Logger) (enabled bool, cfg Config) {
	if log == nil {
		log = slog.Default()
	}
	cfg = DefaultConfig()
	invalid := func(key, value string, def any) {
		log.Warn("invalid boundary loader env var; using default", "key", key, "value", value, "default", def)
	}

	enabled = true
	if v := strings.TrimSpace(getenv("BOUNDARY_LOADER_ENABLED")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			enabled = b
		} else {
			invalid("BOUNDARY_LOADER_ENABLED", v, true)
		}
	}
	if v := strings.TrimSpace(getenv("BOUNDARY_RETRY_HOURS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RetryAfter = time.Duration(n) * time.Hour
		} else {
			invalid("BOUNDARY_RETRY_HOURS", v, 24)
		}
	}
	if v := strings.TrimSpace(getenv("BOUNDARY_MAX_ARCHIVE_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.MaxArchiveBytes = n
		} else {
			invalid("BOUNDARY_MAX_ARCHIVE_BYTES", v, DefaultMaxArchiveBytes)
		}
	}
	if v := strings.TrimSpace(getenv("BOUNDARY_TX_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 && d < leaseTTL {
			cfg.TxTimeout = d
		} else {
			invalid("BOUNDARY_TX_TIMEOUT", v, DefaultTxTimeout.String())
		}
	}
	if v := strings.TrimSpace(getenv("BOUNDARY_COUNTRY_PAUSE_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.CountryPause = time.Duration(n) * time.Millisecond
		} else {
			invalid("BOUNDARY_COUNTRY_PAUSE_MS", v, 1000)
		}
	}
	return enabled, cfg
}

// Status computes the /health `boundaries` object from boundary_sources.
func (m *Manifest) Status(sources map[string]models.BoundarySource) models.BoundaryStatus {
	st := models.BoundaryStatus{Failed: make([]string, 0), ManifestSHA256: m.Checksum()}
	for _, e := range m.Entries {
		src, ok := sources[e.Code]
		if !ok {
			continue
		}
		loaded := src.EntrySHA256 != nil && src.ReenrichedAt != nil
		if loaded {
			st.Loaded++
		}
		// A failure of the current entry is stale once that same entry is
		// loaded: the loader never re-attempts a loaded entry, so the only way
		// to get both is a commit whose acknowledgement was lost (the server
		// committed, the loader saw an error and recorded a failure after it).
		currentLoaded := loaded && *src.EntrySHA256 == e.Hash()
		if src.FailedEntrySHA256 != nil && *src.FailedEntrySHA256 == e.Hash() && !currentLoaded {
			st.Failed = append(st.Failed, e.Code)
		}
	}
	return st
}

// sourceLister is the part of database.BoundaryRepository the status needs.
type sourceLister interface {
	ListBoundarySources(ctx context.Context) (map[string]models.BoundarySource, error)
}

// StatusReporter serves the /health `boundaries` object.
type StatusReporter struct {
	manifest *Manifest
	store    sourceLister
}

// NewStatusReporter builds a reporter over the manifest and boundary_sources.
func NewStatusReporter(m *Manifest, store sourceLister) *StatusReporter {
	return &StatusReporter{manifest: m, store: store}
}

// BoundaryStatus implements handlers.BoundaryStatusSource.
func (r *StatusReporter) BoundaryStatus(ctx context.Context) (*models.BoundaryStatus, error) {
	sources, err := r.store.ListBoundarySources(ctx)
	if err != nil {
		return nil, err
	}
	st := r.manifest.Status(sources)
	return &st, nil
}
