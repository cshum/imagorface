package imagorface

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/cshum/imagor"
	"github.com/cshum/imagor/imagorpath"
	"github.com/cshum/imagor/processor/vipsprocessor"
	"github.com/cshum/imagor/storage/filestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var testDataDir string

func init() {
	_, b, _, _ := runtime.Caller(0)
	testDataDir = filepath.Join(filepath.Dir(b), "testdata")
}

type goldenTest struct {
	name        string
	path        string
	displayFile string // clean filename saved to testdata/ for README embedding
}

// TestGolden runs the full imagorface stack against testdata/people.jpg and
// saves output images to testdata/golden/ (used as regression baseline) and
// to testdata/ with clean names (used as README display images).
//
// On the first run golden files are written; subsequent runs compare against them.
// Delete testdata/golden/ and re-run to regenerate.
func TestGolden(t *testing.T) {
	goldenDir := filepath.Join(testDataDir, "golden")
	resStorage := filestorage.New(goldenDir, filestorage.WithSaveErrIfExists(true))

	detector := NewDetector()
	processor := vipsprocessor.NewProcessor(vipsprocessor.WithDetector(detector))
	app := imagor.New(
		imagor.WithLoaders(filestorage.New(testDataDir)),
		imagor.WithProcessors(processor),
		imagor.WithUnsafe(true),
		imagor.WithLogger(zap.NewNop()),
	)
	require.NoError(t, app.Startup(context.Background()))
	t.Cleanup(func() {
		assert.NoError(t, app.Shutdown(context.Background()))
	})

	tests := []goldenTest{
		{
			name:        "smart crop",
			path:        "500x300/smart/people.jpg",
			displayFile: "demo-smart-crop.jpg",
		},
		{
			name:        "draw detections",
			path:        "500x300/smart/filters:draw_detections()/people.jpg",
			displayFile: "demo-draw-detections.jpg",
		},
		{
			name:        "redact blur default",
			path:        "500x300/smart/filters:redact()/people.jpg",
			displayFile: "demo-redact-blur.jpg",
		},
		{
			name:        "redact pixelate",
			path:        "500x300/smart/filters:redact(pixelate)/people.jpg",
			displayFile: "demo-redact-pixelate.jpg",
		},
		{
			name: "redact blur custom strength",
			path: "500x300/smart/filters:redact(blur,25)/people.jpg",
		},
		{
			name:        "redact black",
			path:        "500x300/smart/filters:redact(black)/people.jpg",
			displayFile: "demo-redact-black.jpg",
		},
		{
			name: "redact white",
			path: "500x300/smart/filters:redact(white)/people.jpg",
		},
		{
			name:        "redact oval blur",
			path:        "500x300/smart/filters:redact_oval()/people.jpg",
			displayFile: "demo-redact-oval.jpg",
		},
		{
			name: "redact oval pixelate",
			path: "500x300/smart/filters:redact_oval(pixelate)/people.jpg",
		},
		{
			name: "redact oval black",
			path: "500x300/smart/filters:redact_oval(black)/people.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/unsafe/%s", tt.path), nil)
			app.ServeHTTP(w, req)
			assert.Equal(t, 200, w.Code)
			body := w.Body.Bytes()

			_ = resStorage.Put(context.Background(), tt.path, imagor.NewBlobFromBytes(body))

			if tt.displayFile != "" {
				displayPath := filepath.Join(testDataDir, tt.displayFile)
				if _, err := os.Stat(displayPath); os.IsNotExist(err) {
					_ = os.WriteFile(displayPath, body, 0644)
				}
			}

			goldenPath := filepath.Join(goldenDir, imagorpath.Normalize(tt.path, nil))
			bc := imagor.NewBlobFromFile(goldenPath)
			buf, err := bc.ReadAll()
			require.NoError(t, err)
			if !reflect.DeepEqual(buf, body) {
				t.Errorf("golden mismatch for %q: got %d bytes, want %d bytes; "+
					"delete testdata/golden/ and re-run to regenerate", tt.path, len(body), len(buf))
			}
		})
	}
}
