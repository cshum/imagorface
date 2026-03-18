// Package imagorface adds pigo-based face detection to imagor's smart crop pipeline.
//
// When the --face-detector flag is set, detected faces replace libvips'
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

// WithFaceDetector is a config.Option that registers the --face-detector flag
// and wires a pigo face detector into any imagor.DetectorSetter processor
// (e.g. vipsprocessor.Processor).
//
// It must be listed after vipsconfig.WithVips in config.CreateServer because
// the applyOptions onion pattern means later entries run after their inner
// neighbours; the vips processor must already exist in app.Processors when
// this option runs.
func WithFaceDetector(fs *flag.FlagSet, cb func() (*zap.Logger, bool)) imagor.Option {
	faceDetect := fs.Bool("face-detector", false,
		"enable pigo face detection for smart crop")
	faceDetectCacheSize := fs.Int("face-detector-cache-size", 0,
		"face detect cache size in number of entries (one per unique source image path). 0 = disabled (default)")
	faceDetectCacheTTL := fs.Duration("face-detector-cache-ttl", 0,
		"face detect cache TTL. 0 = no expiry (default)")
	faceDetectMinSize := fs.Int("face-detector-min-size", 20,
		"minimum face size in pixels on the probe image (default 20)")
	faceDetectMaxSize := fs.Int("face-detector-max-size", 400,
		"maximum face size in pixels on the probe image (default 400)")
	faceDetectMinQuality := fs.Float64("face-detector-min-quality", 5.0,
		"minimum detection quality threshold; lower = more candidates, higher = fewer false positives (default 5.0)")
	faceDetectIoU := fs.Float64("face-detector-iou-threshold", 0.2,
		"intersection-over-union threshold for non-maxima suppression (default 0.2)")
	logger, isDebug := cb()
	var d imagor.Detector
	if *faceDetect {
		d = NewDetector(
			WithLogger(logger),
			WithDebug(isDebug),
			WithCacheSize(*faceDetectCacheSize),
			WithCacheTTL(*faceDetectCacheTTL),
			WithMinSize(*faceDetectMinSize),
			WithMaxSize(*faceDetectMaxSize),
			WithMinQuality(float32(*faceDetectMinQuality)),
			WithIoUThreshold(*faceDetectIoU),
		)
	}
	return imagor.WithDetectors(d)
}
