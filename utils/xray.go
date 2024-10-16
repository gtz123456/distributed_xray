package utils

import "os/exec"

func launchXray() {
	path := "/bin/xray"

	// launch xray

	cmd := exec.Command(path)

	err := cmd.Start()
	if err != nil {
		panic(err)
	}

}
