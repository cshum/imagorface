package main

import (
	"os"

	"github.com/cshum/imagor/config"
	"github.com/cshum/imagor/config/awsconfig"
	"github.com/cshum/imagor/config/gcloudconfig"
	"github.com/cshum/imagor/config/vipsconfig"
	imagorface "github.com/cshum/imagorface"
)

func main() {
	var server = config.CreateServer(
		os.Args[1:],
		vipsconfig.WithVips,
		imagorface.WithFaceDetector,
		awsconfig.WithAWS,
		gcloudconfig.WithGCloud,
	)
	if server != nil {
		server.Run()
	}
}
