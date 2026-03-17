package imagorface

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/cshum/imagor"
	"github.com/dgraph-io/ristretto/v2"
	pigo "github.com/esimov/pigo/core"
	"go.uber.org/zap"
)

//go:embed cascade/facefinder
var defaultCascade []byte

// Detector detects faces using pigo's PICO cascade classifier.
// Create with NewDetector or NewDetectorWithCascade; the classifier is
// initialised lazily in Startup so that construction never returns an error.
type Detector struct {
	cascade      []byte
	classifier   *pigo.Pigo
	minSize      int
	maxSize      int
	minQuality   float32
	iouThreshold float64
	cacheSize    int
	cacheTTL     time.Duration
	cache        *ristretto.Cache[string, []imagor.DetectorRegion]
	logger       *zap.Logger
	debug        bool
}

// NewDetector creates a Detector using the embedded facefinder cascade.
// The cascade is parsed during Startup, not here.
func NewDetector(opts ...Option) *Detector {
	return NewDetectorWithCascade(defaultCascade, opts...)
}

// NewDetectorWithCascade creates a Detector using the provided cascade bytes.
func NewDetectorWithCascade(cascade []byte, opts ...Option) *Detector {
	d := &Detector{
		cascade:      cascade,
		minSize:      20,
		maxSize:      400,
		minQuality:   5.0,
		iouThreshold: 0.2,
		logger:       zap.NewNop(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Startup parses the cascade file. Called by vipsprocessor during server startup.
// Returns an error if the cascade is corrupt; vipsprocessor will log a warning
// and nil out the detector so the server starts normally without face detection.
func (d *Detector) Startup(_ context.Context) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("face detector: cascade parse panic: %v", r)
		}
	}()
	classifier, err := pigo.NewPigo().Unpack(d.cascade)
	if err != nil {
		return err
	}
	d.classifier = classifier
	d.logger.Debug("face detector initialised", zap.Int("cascade_bytes", len(d.cascade)))
	if d.cacheSize > 0 {
		cache, err := ristretto.NewCache[string, []imagor.DetectorRegion](&ristretto.Config[string, []imagor.DetectorRegion]{
			NumCounters: int64(d.cacheSize) * 10,
			MaxCost:     int64(d.cacheSize),
			BufferItems: 64,
		})
		if err != nil {
			return err
		}
		d.cache = cache
	}
	return nil
}

// Shutdown closes the detect cache if one was initialised.
func (d *Detector) Shutdown(_ context.Context) error {
	if d.cache != nil {
		d.cache.Close()
		d.cache = nil
	}
	return nil
}

// Detect implements imagor.Detector.
//
// imagePath identifies the source image and is used as a cache key when the
// detect cache is enabled (WithCacheSize > 0). Pass an empty string to skip
// the cache for a particular call.
//
// blob must be a BlobTypeMemory blob carrying row-major sRGB/sRGBA pixels.
// Dimensions are retrieved via blob.Memory().
//
// Returned regions are normalised to [0.0, 1.0] relative to width / height.
func (d *Detector) Detect(_ context.Context, imagePath string, blob *imagor.Blob) (regions []imagor.DetectorRegion, err error) {
	buf, width, height, bands, ok := blob.Memory()
	if !ok {
		return
	}
	if d.debug {
		start := time.Now()
		defer func() {
			d.logger.Debug("face detect",
				zap.Int("width", width),
				zap.Int("height", height),
				zap.Int("regions", len(regions)),
				zap.Duration("took", time.Since(start)),
			)
		}()
	}
	if d.cache != nil && imagePath != "" {
		if cached, ok := d.cache.Get(imagePath); ok {
			return cached, nil
		}
	}
	if d.classifier == nil || bands < 3 || len(buf) != width*height*bands {
		return
	}
	pixels := toGrayscale(buf, width, height, bands)
	maxSize := min(d.maxSize, min(width, height))
	cParams := pigo.CascadeParams{
		MinSize:     d.minSize,
		MaxSize:     maxSize,
		ShiftFactor: 0.1,
		ScaleFactor: 1.1,
		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   height,
			Cols:   width,
			Dim:    width,
		},
	}
	dets := d.classifier.RunCascade(cParams, 0.0)
	dets = d.classifier.ClusterDetections(dets, d.iouThreshold)

	for _, det := range dets {
		if det.Q < d.minQuality {
			continue
		}
		half := float64(det.Scale) / 2
		left := math.Max(0, float64(det.Col)-half) / float64(width)
		top := math.Max(0, float64(det.Row)-half) / float64(height)
		right := math.Min(float64(width), float64(det.Col)+half) / float64(width)
		bottom := math.Min(float64(height), float64(det.Row)+half) / float64(height)
		if right > left && bottom > top {
			regions = append(regions, imagor.DetectorRegion{
				Left:   left,
				Top:    top,
				Right:  right,
				Bottom: bottom,
				Score:  math.Round(float64(det.Q)*100) / 100,
				Name:   "face",
			})
		}
	}
	if d.cache != nil && imagePath != "" {
		if d.cacheTTL > 0 {
			d.cache.SetWithTTL(imagePath, regions, 1, d.cacheTTL)
		} else {
			d.cache.Set(imagePath, regions, 1)
		}
		d.cache.Wait()
	}
	return
}

// toGrayscale converts a row-major sRGB(A) pixel buffer to a flat grayscale
// []uint8 using ITU-R BT.601 luminance weights.
func toGrayscale(buf []uint8, width, height, bands int) []uint8 {
	pixels := make([]uint8, width*height)
	for i := range width * height {
		base := i * bands
		r := uint32(buf[base])
		g := uint32(buf[base+1])
		b := uint32(buf[base+2])
		// ITU-R BT.601 luminance: 0.299*R + 0.587*G + 0.114*B
		pixels[i] = uint8((299*r + 587*g + 114*b) / 1000)
	}
	return pixels
}
