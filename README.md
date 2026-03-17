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

imagorface adds the `detections` filter to the imagor pipeline. See [imagor filters](https://github.com/cshum/imagor#filters) for the full filter reference.

- `detections([color])` draws bounding box outlines for all detected regions onto the image. Intended for visual debugging.

Example:
```
http://localhost:8000/unsafe/filters:detections(ff0000)/https://example.com/photo.jpg
```

### Metadata

imagorface exposes detected face regions through imagor's metadata endpoint. Each region is returned in absolute pixels relative to the output image dimensions, along with a detection score and label.

To use the metadata endpoint, add `/meta` right after the URL signature hash before the image operations:

```
http://localhost:8000/unsafe/meta/https://example.com/photo.jpg
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

### Configuration

Configuration options specific to imagorface. Please see [imagor configuration](https://github.com/cshum/imagor#configuration) for all existing options available.

```
  -face-detect
        enable pigo face detection for smart crop
```
