# imagorface

[![Test Status](https://github.com/cshum/imagorface/workflows/test/badge.svg)](https://github.com/cshum/imagorface/actions/workflows/test.yml)
[![Codecov](https://img.shields.io/codecov/c/github/cshum/imagorface)](https://codecov.io/gh/cshum/imagorface)
[![Docker Hub](https://img.shields.io/badge/docker-shumc/imagorface-blue.svg)](https://hub.docker.com/r/shumc/imagorface/)

imagorface brings face detection capability through [pigo](https://github.com/esimov/pigo), built on the foundations of [imagor](https://github.com/cshum/imagor) — a fast, secure image processing server and Go library, using libvips.

imagorface uses pigo's pure-Go PICO cascade classifier to detect faces in an image. Detected face regions replace libvips' attention heuristic as the smart crop anchor, producing face-centred crops. It also exposes detected regions through imagor's [metadata endpoint](#metadata) and the [`detections`](#filters) filter for visual inspection.

imagorface implements the imagor [`Detector` interface](https://github.com/cshum/imagor/blob/master/detector.go), wiring into imagor's [loader, storage and result storage](https://github.com/cshum/imagor#loader-storage-and-result-storage), which supports HTTP(s), File System, AWS S3 and Google Cloud Storage out of the box.

This also aims to be a reference project demonstrating imagor extension.

### Quick Start

```bash
docker run -p 8000:8000 shumc/imagorface -imagor-unsafe -face-detect
```

Enable face-centred smart crop:
```
http://localhost:8000/unsafe/300x300/smart/https://example.com/group-photo.jpg
```

Visualise detected regions:
```
http://localhost:8000/unsafe/filters:detections()/https://example.com/group-photo.jpg
```

### Smart Crop

When `-face-detect` is enabled, imagorface runs face detection before the crop step. If one or more faces are found, their bounding boxes are used as the focal region for [imagor's smart crop](https://github.com/cshum/imagor#image-endpoint), replacing the default libvips attention-based heuristic. When no faces are found, imagor falls back to the standard attention crop.

Face detection runs on a downscaled greyscale probe derived from the raw decoded pixels, keeping overhead minimal relative to the subsequent libvips operations.

### Filters

imagorface enables the following filters in the imagor pipeline. See [imagor filters](https://github.com/cshum/imagor#filters) for the full filter reference.

- `detections()` **debug only** — draws colour-coded bounding boxes for all detected regions. Each class name is automatically assigned a distinct colour via hash-based palette for visual inspection.

```
http://localhost:8000/unsafe/filters:detections()/https://example.com/photo.jpg
```

- `redact([mode[, strength]])` obscures all detected regions for privacy/anonymisation (e.g. GDPR face blurring, legal document redaction). No-op when no regions are detected. Skips animated images.
  - `mode` — `blur` (default), `pixelate`, or any color name/hex for solid fill (e.g. `black`, `white`, `ff0000`)
  - `strength` — blur sigma (default 15) or pixelate block size in pixels (default 10). Not used for solid color mode.

```
# Blur detected faces (default)
http://localhost:8000/unsafe/filters:redact()/https://example.com/photo.jpg

# Pixelate detected faces
http://localhost:8000/unsafe/filters:redact(pixelate)/https://example.com/photo.jpg

# Blur with custom strength
http://localhost:8000/unsafe/filters:redact(blur,25)/https://example.com/photo.jpg

# Solid black fill (most common for legal/compliance redaction)
http://localhost:8000/unsafe/filters:redact(black)/https://example.com/photo.jpg

# Solid white fill
http://localhost:8000/unsafe/filters:redact(white)/https://example.com/photo.jpg

# Custom color fill
http://localhost:8000/unsafe/filters:redact(ff0000)/https://example.com/photo.jpg
```

### Metadata

imagorface exposes detected face regions through imagor's metadata endpoint. Each region is returned in absolute pixels relative to the output image dimensions, along with a detection score and label.

To use the metadata endpoint, add `/meta` right after the URL signature hash before the image operations. Detection only runs when the URL semantically requests it — `smart`, `detections()`, or `redact()` to trigger detection:

```
# Runs detection, returns detected_regions
http://localhost:8000/unsafe/meta/filters:detections()/https://example.com/photo.jpg
http://localhost:8000/unsafe/meta/smart/https://example.com/photo.jpg
```

Response includes a `detected_regions` array:

```json
{
  "format": "jpeg",
  "content_type": "image/jpeg",
  "width": 800,
  "height": 600,
  "detected_regions": [
    { "left": 120, "top": 45, "right": 280, "bottom": 205, "score": 12.34, "name": "face" },
    { "left": 350, "top": 60, "right": 490, "bottom": 200, "score": 9.10, "name": "face" }
  ]
}
```

`score` is the raw pigo detection quality (higher is more confident). `name` is `"face"` for all regions returned by this detector.

### Go Library

imagorface can be used as a Go library alongside imagor:

```go
import (
    "github.com/cshum/imagor/config"
    "github.com/cshum/imagor/config/vipsconfig"
    imagorface "github.com/cshum/imagorface"
)

server := config.CreateServer(
    os.Args[1:],
    vipsconfig.WithVips,
    imagorface.WithFaceDetector, // must be listed after WithVips
)
if server != nil {
    server.Run()
}
```

Or construct a detector directly and pass it to `vipsprocessor`:

```go
import (
    "github.com/cshum/imagor/processor/vipsprocessor"
    imagorface "github.com/cshum/imagorface"
)

detector := imagorface.NewDetector(
    imagorface.WithMinSize(20),
    imagorface.WithMinQuality(5.0),
)

processor := vipsprocessor.NewProcessor(
    vipsprocessor.WithDetector(detector),
)
```

### Face Detect Cache

imagorface maintains an in-memory cache of detection results, keyed by source image path. This avoids re-running the pigo cascade on the same source image across repeated requests — smart crop, `detections()`, and `redact()` all benefit.

The cache stores `[]imagor.Region` slices keyed by image path and is backed by [ristretto](https://github.com/dgraph-io/ristretto) with LRU eviction and a configurable entry count.

```dotenv
FACE_DETECT_CACHE_SIZE=500  # Max number of cached detection results. Default 0 = disabled
FACE_DETECT_CACHE_TTL=1h    # Cache entry TTL. Default 0 = no expiry (LRU eviction only)
```

**When to use:**
- Enable when the same source images are requested repeatedly (e.g. a product catalogue where the same images are cropped at multiple sizes). The first request runs pigo; all subsequent requests for the same path return the cached regions instantly.
- Set `FACE_DETECT_CACHE_TTL` if source images may change at the same path (e.g. mutable assets). Without a TTL, stale detection results are served until evicted by memory pressure or process restart.
- Leave disabled (default) if source image paths are highly varied or user-supplied, as caching provides no benefit.
- The `detections()` visual debug filter always bypasses the cache (it passes an empty path) since it is not a hot path.

### Configuration

Configuration options specific to imagorface. Please see [imagor configuration](https://github.com/cshum/imagor#configuration) for all existing options available.

```
  -face-detect
        enable pigo face detection for smart crop
  -face-detect-min-size int
        minimum face size in pixels on the probe image (default 20)
  -face-detect-max-size int
        maximum face size in pixels on the probe image (default 400)
  -face-detect-min-quality float
        minimum detection quality threshold; lower = more candidates, higher = fewer false positives (default 5.0)
  -face-detect-iou-threshold float
        intersection-over-union threshold for non-maxima suppression (default 0.2)
  -face-detect-cache-size int
        face detect cache size in number of entries (one per unique source image path). 0 = disabled (default)
  -face-detect-cache-ttl duration
        face detect cache TTL. 0 = no expiry (default)
```

Environment variable equivalents (uppercase, hyphens → underscores):
```dotenv
FACE_DETECT=1
FACE_DETECT_MIN_SIZE=20
FACE_DETECT_MAX_SIZE=400
FACE_DETECT_MIN_QUALITY=5.0
FACE_DETECT_IOU_THRESHOLD=0.2
FACE_DETECT_CACHE_SIZE=500
FACE_DETECT_CACHE_TTL=1h
```

