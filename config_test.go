package imagorface

import (
	"testing"
	"time"

	"github.com/cshum/imagor"
	"github.com/cshum/imagor/config"
	"github.com/cshum/imagor/config/vipsconfig"
	"github.com/cshum/imagor/processor/vipsprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Run("face-detector disabled by default", func(t *testing.T) {
		srv := config.CreateServer([]string{}, vipsconfig.WithVips, WithFaceDetector)
		app := srv.App.(*imagor.Imagor)
		proc := app.Processors[0].(*vipsprocessor.Processor)
		assert.Nil(t, proc.Detector,
			"detector must be nil when -face-detector is not set")
	})

	t.Run("face-detector enabled wires detector", func(t *testing.T) {
		srv := config.CreateServer([]string{
			"-face-detector",
		}, vipsconfig.WithVips, WithFaceDetector)
		app := srv.App.(*imagor.Imagor)
		proc := app.Processors[0].(*vipsprocessor.Processor)
		require.NotNil(t, proc.Detector,
			"detector must be set when -face-detector is passed")
		d, ok := proc.Detector.(*Detector)
		require.True(t, ok, "detector must be *imagorface.Detector")
		// Defaults
		assert.Equal(t, 20, d.minSize)
		assert.Equal(t, 400, d.maxSize)
		assert.Equal(t, float32(5.0), d.minQuality)
		assert.Equal(t, 0.2, d.iouThreshold)
		assert.Equal(t, 0, d.cacheSize)
		assert.Equal(t, time.Duration(0), d.cacheTTL)
	})

	t.Run("all flags wired correctly", func(t *testing.T) {
		srv := config.CreateServer([]string{
			"-face-detector",
			"-face-detector-min-size", "30",
			"-face-detector-max-size", "300",
			"-face-detector-min-quality", "8.5",
			"-face-detector-iou-threshold", "0.35",
			"-face-detector-cache-size", "500",
			"-face-detector-cache-ttl", "1h",
		}, vipsconfig.WithVips, WithFaceDetector)
		app := srv.App.(*imagor.Imagor)
		proc := app.Processors[0].(*vipsprocessor.Processor)
		require.NotNil(t, proc.Detector)
		d, ok := proc.Detector.(*Detector)
		require.True(t, ok)
		assert.Equal(t, 30, d.minSize)
		assert.Equal(t, 300, d.maxSize)
		assert.Equal(t, float32(8.5), d.minQuality)
		assert.Equal(t, 0.35, d.iouThreshold)
		assert.Equal(t, 500, d.cacheSize)
		assert.Equal(t, time.Hour, d.cacheTTL)
	})
}
