package registry

type Registration struct {
	ServiceName      ServiceName
	ServiceURL       string
	RequiredServices []ServiceName
	ServiceUpdateURL string
}

type ServiceName string

const (
	LogService   = ServiceName("LogService")
	ShellService = ServiceName("ShellService")
	NodeService  = ServiceName("NodeService")
	UserService  = ServiceName("UserService")
)

type patchEntry struct {
	Name ServiceName
	URL  string
}

type patch struct {
	Added   []patchEntry
	Removed []patchEntry
}
