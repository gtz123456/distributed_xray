package utils

import (
	"fmt"
	"os/exec"
	"runtime"
)

func LaunchXray() error {
	path := "./app/bin/xray"

	// add arm64 support
	if runtime.GOARCH == "arm64" {
		path += "_arm"
	}

	// print system path
	fmt.Println("System path: " + path)

	// launch xray
	fmt.Println("Launching xray " + path)
	cmd := exec.Command(path)

	err := cmd.Start()
	if err != nil {
		fmt.Println("Error launching xray: " + err.Error())
		return err
	}

	return nil
}

func ConfigXray(realitykey string) {
	// TODO: configuration logic for xray
}
