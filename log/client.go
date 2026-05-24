package log

import (
	"bytes"
	"fmt"
	"go-distributed/registry"
	"io"
	stlog "log"
	"net/http"
)

func SetClientLogger(serviceURL string, r registry.Registration) {
	prefix := fmt.Sprintf("[%v", r.ServiceName)
	if r.ServiceID != "" {
		prefix += fmt.Sprintf("|%v", r.ServiceID)
	}
	if r.PublicIP != "" {
		prefix += fmt.Sprintf("|%v", r.PublicIP)
	}
	if r.PublicIPv6 != "" {
		prefix += fmt.Sprintf("|%v", r.PublicIPv6)
	}
	prefix += "] - "

	stlog.SetPrefix(prefix)
	stlog.SetFlags(0)
	stlog.SetOutput(&clientLogger{url: serviceURL})
}

type clientLogger struct {
	url string
}

var _ io.Writer = (*clientLogger)(nil)

func (cl clientLogger) Write(data []byte) (int, error) {
	b := bytes.NewBuffer([]byte(data))
	res, err := http.Post(cl.url+"/log", "text/plain", b)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Failed to send log message. Service responed with code %v", res.StatusCode)
	}
	return len(data), nil
}
