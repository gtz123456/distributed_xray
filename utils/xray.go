package utils

import (
	"os/exec"
	"runtime"
)

func LaunchXray() {
	path := "./bin/xray"

	// add arm64 support
	if runtime.GOARCH == "arm64" {
		path += "_arm"
	}

	// launch xray

	cmd := exec.Command(path)

	err := cmd.Start()
	if err != nil {
		panic(err)
	}
}

func ConfigXray(realitykey string) {
	// configuration logic for xray
}
