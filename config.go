// Package imagorface adds pigo-based face detection to imagor's smart crop pipeline.
//
// When the --face-detect flag is set, detected faces replace libvips'
// attention heuristic as the smart crop anchor, producing face-centred crops.
//
// Usage in a custom server binary:
//
//	config.CreateServer(os.Args[1:],
//	    vipsconfig.WithVips,
//	    imagorface.WithFaceDetector, // must be listed after WithVips
//	    awsconfig.WithAWS,
//	    gcloudconfig.WithGCloud,
//	)
package imagorface

import (
	"flag"

	"github.com/cshum/imagor"
	"go.uber.org/zap"
)

// WithFaceDetector is a config.Option that registers the --face-detect flag
// and wires a pigo face detector into any imagor.DetectorSetter processor
// (e.g. vipsprocessor.Processor).
//
// It must be listed after vipsconfig.WithVips in config.CreateServer because
// the applyOptions onion pattern means later entries run after their inner
// neighbours; the vips processor must already exist in app.Processors when
// this option runs.
func WithFaceDetector(fs *flag.FlagSet, cb func() (*zap.Logger, bool)) imagor.Option {
	faceDetect := fs.Bool("face-detect", false,
		"enable pigo face detection for smart crop")
	faceDetectCacheSize := fs.Int("face-detect-cache-size", 0,
		"face detect cache size in number of entries (one per unique source image path). 0 = disabled (default)")
	faceDetectCacheTTL := fs.Duration("face-detect-cache-ttl", 0,
		"face detect cache TTL. 0 = no expiry (default)")
	logger, isDebug := cb()
	var d imagor.Detector
	if *faceDetect {
		d = NewDetector(
			WithLogger(logger),
			WithDebug(isDebug),
			WithCacheSize(*faceDetectCacheSize),
			WithCacheTTL(*faceDetectCacheTTL),
		)
	}
	return imagor.WithDetector(d)
}
