package imagorface

import (
	"time"

	"go.uber.org/zap"
)

// Option is a functional option for Detector.
type Option func(*Detector)

// WithMinSize sets the minimum face size in pixels on the probe image (default 20).
func WithMinSize(size int) Option {
	return func(d *Detector) { d.minSize = size }
}

// WithMaxSize sets the maximum face size in pixels on the probe image (default 400).
func WithMaxSize(size int) Option {
	return func(d *Detector) { d.maxSize = size }
}

// WithMinQuality sets the minimum detection quality threshold (default 5.0).
// Lower values produce more candidates; raise it to reduce false positives.
func WithMinQuality(q float32) Option {
	return func(d *Detector) { d.minQuality = q }
}

// WithIoUThreshold sets the intersection-over-union threshold used by
// ClusterDetections non-maxima suppression (default 0.2).
func WithIoUThreshold(t float64) Option {
	return func(d *Detector) { d.iouThreshold = t }
}

// WithLogger sets the logger for the detector (default: no-op).
func WithLogger(logger *zap.Logger) Option {
	return func(d *Detector) {
		if logger != nil {
			d.logger = logger
		}
	}
}

// WithDebug enables debug logging (default: false).
func WithDebug(debug bool) Option {
	return func(d *Detector) { d.debug = debug }
}

// WithCacheSize sets the detect result cache size in number of entries.
// Each unique source image path counts as one entry (cost 1).
// 0 disables the cache (default).
func WithCacheSize(n int) Option {
	return func(d *Detector) { d.cacheSize = n }
}

// WithCacheTTL sets the TTL for detect result cache entries.
// After expiry, detection re-runs on the next request.
// 0 means no expiry (default).
func WithCacheTTL(ttl time.Duration) Option {
	return func(d *Detector) { d.cacheTTL = ttl }
}
