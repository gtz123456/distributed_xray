package registry

import (
	"go-distributed/utils"
	"os"
)

type Registration struct {
	ServiceName      ServiceName
	ServiceURL       string
	ServiceID        string
	PublicIP         string
	RequiredServices []ServiceName
	ServiceUpdateURL string
}

type ServiceName string

const (
	LogService   = ServiceName("LogService")
	ShellService = ServiceName("ShellService")
	NodeService  = ServiceName("NodeService")
	WebService   = ServiceName("WebService")
)

type patchEntry struct {
	Name ServiceName
	URL  string
}

type patch struct {
	Added   []patchEntry `json:"added"`
	Removed []patchEntry `json:"removed"`
}

var ServerIP string
var ServerPort string
var ServerURL string

func init() {
	utils.LoadEnv()
	// Load environment variables
	// For registry server, the ServerIP and ServerPort should be the addr it listens on, such as localhost:3000 or [::]:80
	ServerIP = os.Getenv("Registry_IP")

	ServerPort = os.Getenv("Registry_Port")
	if ServerPort == "" {
		ServerPort = "80"
	}

	ServerURL = "http://" + ServerIP + ":" + ServerPort + "/services"
}
