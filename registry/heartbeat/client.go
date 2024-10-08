package heartbeat

import (
	"fmt"
	"net/http"
)

type BasicHeartbeat struct {
	URL string
}

func (b *BasicHeartbeat) SendHeartbeat() error {
	// fmt.Println("Sending heartbeat to registry")
	res, err := http.Post(b.URL, "application/json", nil)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send heartbeat. Registry service responed with status code %v", res.StatusCode)
	}
	// fmt.Println("Heartbeat sent")
	return nil
}

func NewBasicHeartbeat(url string) HeartbeatStrategy {
	return &BasicHeartbeat{URL: url}
}

/*
func GetCPUUsage() (string, error) {
	return "10%", nil // TODO: Implement this
}

func GetMemUsage() (string, error) {
	return "20%", nil // TODO: Implement this
}

func (i *InfoHeartbeat) SendHeartbeat() error {
	CPUUsage, err := GetCPUUsage()
	if err != nil {
		return err
	}
	MemUsage, err := GetMemUsage()
	if err != nil {
		return err
	}
	heartbeat := ServerInfo{
		CPUUsage:    CPUUsage,
		MemoryUsage: MemUsage,
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	err = enc.Encode(heartbeat)
	if err != nil {
		return err
	}
	res, err := http.Post(i.url, "application/json", &b)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send heartbeat. Registry service responed with status code %v", res.StatusCode)
	}
	return nil
}
*/
