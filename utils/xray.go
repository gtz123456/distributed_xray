package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func LaunchXray() error {
	path := os.Getenv("XRAY_PATH")

	// add arm64 support
	if runtime.GOARCH == "arm64" {
		path += "_arm"
	}

	// config.json is located next to the xray binary
	configPath := filepath.Join(filepath.Dir(path), "config.json")

	// print system path
	fmt.Println("System path: " + path)

	// launch xray
	fmt.Println("Launching xray " + path + " with config " + configPath)
	cmd := exec.Command(path, "run", "-c", configPath)

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
