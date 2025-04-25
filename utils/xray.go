package utils

import (
	"fmt"
	"os/exec"
	"runtime"
)

func LaunchXray() {
	path := "/app/bin/xray"

	// add arm64 support
	if runtime.GOARCH == "arm64" {
		path += "_arm"
	}

	// launch xray
	fmt.Println("Launching xray" + path)
	cmd := exec.Command(path)

	err := cmd.Start()
	if err != nil {
		fmt.Println("Error launching xray: " + err.Error())
		panic(err)
	}
}

func ConfigXray(realitykey string) {
	// configuration logic for xray
}
