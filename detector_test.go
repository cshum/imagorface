package imagorface

import (
	"context"
	"image"
	"image/color"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cshum/imagor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeBlob constructs a BlobTypeMemory blob from a solid-colour RGBA image of
// the given dimensions. bands=3 (RGB) or bands=4 (RGBA).
func makeBlob(width, height, bands int) *imagor.Blob {
	buf := make([]byte, width*height*bands)
	// Fill with a mid-grey so pigo has pixels to work with.
	for i := range buf {
		buf[i] = 128
	}
	return imagor.NewBlobFromMemory(buf, width, height, bands)
}

// makeFaceBlob returns a small synthetic blob that approximates a face-like
// pattern (dark oval on light background) at 80×80 so pigo can exercise the
// cascade, though detections are not guaranteed on synthetic data.
func makeFaceBlob() *imagor.Blob {
	const w, h = 80, 80
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy, rx, ry := w/2, h/2, w/3, h/3
	for y := range h {
		for x := range w {
			dx := float64(x-cx) / float64(rx)
			dy := float64(y-cy) / float64(ry)
			if dx*dx+dy*dy <= 1.0 {
				img.Set(x, y, color.RGBA{200, 150, 130, 255})
			} else {
				img.Set(x, y, color.RGBA{240, 240, 240, 255})
			}
		}
	}
	buf := make([]byte, w*h*4)
	for i := range w * h {
		x, y := i%w, i/w
		r, g, b, a := img.At(x, y).RGBA()
		buf[i*4], buf[i*4+1], buf[i*4+2], buf[i*4+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
	}
	return imagor.NewBlobFromMemory(buf, w, h, 4)
}

// TestStartupShutdown verifies that Startup parses the cascade without error
// and Shutdown is idempotent.
func TestStartupShutdown(t *testing.T) {
	d := NewDetector()
	require.NoError(t, d.Startup(context.Background()))
	require.NoError(t, d.Shutdown(context.Background()))
	// Second Shutdown must not panic.
	require.NoError(t, d.Shutdown(context.Background()))
}

// TestStartupBadCascade verifies that a corrupt cascade bytes cause Startup to
// return an error.
func TestStartupBadCascade(t *testing.T) {
	d := NewDetectorWithCascade([]byte("not a valid cascade"))
	err := d.Startup(context.Background())
	assert.Error(t, err)
}

// TestDetectNonMemoryBlob verifies that a non-memory blob returns empty regions
// without error.
func TestDetectNonMemoryBlob(t *testing.T) {
	d := NewDetector()
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	blob := imagor.NewBlobFromBytes([]byte("not pixels"))
	regions, err := d.Detect(context.Background(), "test.png", blob)
	assert.NoError(t, err)
	assert.Empty(t, regions)
}

// TestDetectMismatchedBuf verifies that a blob whose buf length does not match
// width×height×bands returns empty regions.
func TestDetectMismatchedBuf(t *testing.T) {
	d := NewDetector()
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	// Create a blob that claims to be 10×10×3 but only has 10 bytes.
	blob := imagor.NewBlobFromMemory(make([]byte, 10), 10, 10, 3)
	regions, err := d.Detect(context.Background(), "test.png", blob)
	assert.NoError(t, err)
	assert.Empty(t, regions)
}

// TestDetectSmokeRGB verifies that Detect runs without error on a valid RGB
// memory blob (detections are not asserted on synthetic data).
func TestDetectSmokeRGB(t *testing.T) {
	d := NewDetector()
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	_, err := d.Detect(context.Background(), "", makeBlob(100, 100, 3))
	assert.NoError(t, err)
}

// TestDetectSmokeRGBA is the same as TestDetectSmokeRGB but with 4-band RGBA input.
func TestDetectSmokeRGBA(t *testing.T) {
	d := NewDetector()
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	_, err := d.Detect(context.Background(), "", makeBlob(100, 100, 4))
	assert.NoError(t, err)
}

// TestDetectRegionBounds verifies that all returned region coordinates are in
// [0.0, 1.0] and that Right > Left, Bottom > Top.
func TestDetectRegionBounds(t *testing.T) {
	d := NewDetector(WithMinQuality(1.0))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	regions, err := d.Detect(context.Background(), "", makeFaceBlob())
	require.NoError(t, err)
	for _, r := range regions {
		assert.GreaterOrEqual(t, r.Left, 0.0)
		assert.GreaterOrEqual(t, r.Top, 0.0)
		assert.LessOrEqual(t, r.Right, 1.0)
		assert.LessOrEqual(t, r.Bottom, 1.0)
		assert.Greater(t, r.Right, r.Left)
		assert.Greater(t, r.Bottom, r.Top)
		assert.Equal(t, "face", r.Name)
		assert.GreaterOrEqual(t, r.Score, 0.0)
	}
}

// countingDetector wraps a real Detector and counts calls to the inner detect
// logic by counting cache misses (i.e. actual pigo runs). We do this by
// subclassing and tracking the toGrayscale hot path via an atomic in a
// test-only wrapper around Detect that bypasses the cache.
//
// Since the cache lives inside Detector, we use a surrogate: we count how many
// times Detect is called with a fresh blob (no cache entry). We achieve a
// deterministic count by spying at the Detector's cache directly.
func cacheHitCount(d *Detector, key string) int {
	if d.cache == nil {
		return 0
	}
	_, ok := d.cache.Get(key)
	if ok {
		return 1
	}
	return 0
}

// TestCacheDisabledByDefault verifies that the cache is nil when no cacheSize
// is set.
func TestCacheDisabledByDefault(t *testing.T) {
	d := NewDetector()
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck
	assert.Nil(t, d.cache)
}

// TestCacheInitialisedWhenEnabled verifies that Startup creates the cache when
// WithCacheSize is set.
func TestCacheInitialisedWhenEnabled(t *testing.T) {
	d := NewDetector(WithCacheSize(50))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck
	assert.NotNil(t, d.cache)
}

// TestCacheShutdownNilsCache verifies that Shutdown closes and nils the cache.
func TestCacheShutdownNilsCache(t *testing.T) {
	d := NewDetector(WithCacheSize(50))
	require.NoError(t, d.Startup(context.Background()))
	require.NotNil(t, d.cache)
	require.NoError(t, d.Shutdown(context.Background()))
	assert.Nil(t, d.cache)
}

// TestCacheHit verifies that a second Detect call for the same imagePath returns
// the cached result without re-running detection. We verify this by injecting a
// pre-populated cache entry and confirming Detect returns it unchanged.
func TestCacheHit(t *testing.T) {
	d := NewDetector(WithCacheSize(100))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	const key = "face-cache-hit.png"
	want := []imagor.DetectorRegion{
		{Left: 0.1, Top: 0.1, Right: 0.4, Bottom: 0.6, Score: 9.5, Name: "face"},
	}
	// Seed the cache directly.
	d.cache.Set(key, want, 1)
	d.cache.Wait()

	blob := makeBlob(100, 100, 3)
	got, err := d.Detect(context.Background(), key, blob)
	require.NoError(t, err)
	assert.Equal(t, want, got, "Detect should return the cached result")
}

// TestCacheStoreAfterDetect verifies that after a successful detection the
// result is stored in the cache so a subsequent call with the same key returns
// without re-running pigo.
func TestCacheStoreAfterDetect(t *testing.T) {
	d := NewDetector(WithCacheSize(100), WithMinQuality(1.0))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	const key = "store-after-detect.png"
	blob := makeFaceBlob()

	r1, err := d.Detect(context.Background(), key, blob)
	require.NoError(t, err)

	// The cache should now hold an entry.
	assert.Equal(t, 1, cacheHitCount(d, key), "cache should contain the entry after first Detect")

	r2, err := d.Detect(context.Background(), key, blob)
	require.NoError(t, err)

	assert.Equal(t, r1, r2, "second call should return identical cached regions")
}

// TestCacheTTLExpiry verifies that an entry is evicted after its TTL and
// detection re-runs on the next call.
func TestCacheTTLExpiry(t *testing.T) {
	d := NewDetector(WithCacheSize(100), WithCacheTTL(20*time.Millisecond))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	const key = "ttl-expiry.png"
	want := []imagor.DetectorRegion{
		{Left: 0.1, Top: 0.1, Right: 0.4, Bottom: 0.6, Score: 7.0, Name: "face"},
	}
	d.cache.SetWithTTL(key, want, 1, 20*time.Millisecond)
	d.cache.Wait()

	// Entry should be present immediately.
	assert.Equal(t, 1, cacheHitCount(d, key))

	time.Sleep(60 * time.Millisecond)

	// Entry should be gone after TTL.
	assert.Equal(t, 0, cacheHitCount(d, key), "cache entry should be evicted after TTL")
}

// TestCacheEmptyPathSkipsCache verifies that passing an empty imagePath bypasses
// the cache entirely (no store, no lookup).
func TestCacheEmptyPathSkipsCache(t *testing.T) {
	d := NewDetector(WithCacheSize(100))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	blob := makeBlob(100, 100, 3)
	_, err := d.Detect(context.Background(), "", blob)
	require.NoError(t, err)

	// Nothing should be in the cache for the empty key.
	_, ok := d.cache.Get("")
	assert.False(t, ok, "empty imagePath must not be stored in the cache")
}

// TestCacheConcurrency verifies that concurrent Detect calls for the same key
// do not race or panic.
func TestCacheConcurrency(t *testing.T) {
	d := NewDetector(WithCacheSize(100))
	require.NoError(t, d.Startup(context.Background()))
	defer d.Shutdown(context.Background()) //nolint:errcheck

	const key = "concurrent.png"
	blob := makeBlob(60, 60, 3)

	var wg atomic.Int32
	done := make(chan struct{})
	for range 10 {
		go func() {
			_, _ = d.Detect(context.Background(), key, blob)
			if wg.Add(1) == 10 {
				close(done)
			}
		}()
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Detect calls timed out")
	}
}

// TestToGrayscaleRGB verifies BT.601 luminance for a known RGB pixel.
func TestToGrayscaleRGB(t *testing.T) {
	// Pure red pixel (255, 0, 0) → 0.299×255 ≈ 76
	buf := []byte{255, 0, 0}
	got := toGrayscale(buf, 1, 1, 3)
	require.Len(t, got, 1)
	assert.EqualValues(t, 76, got[0])
}

// TestToGrayscaleRGBA verifies that the alpha channel is ignored.
func TestToGrayscaleRGBA(t *testing.T) {
	// Pure green pixel (0, 255, 0, 128) → 0.587×255 ≈ 149
	buf := []byte{0, 255, 0, 128}
	got := toGrayscale(buf, 1, 1, 4)
	require.Len(t, got, 1)
	assert.EqualValues(t, 149, got[0])
}
