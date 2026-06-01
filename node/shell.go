package node

import (
	"go-distributed/utils"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var shellUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow any origin since this will be accessed via proxy
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (sh *nodeHandler) handleShell(w http.ResponseWriter, r *http.Request) {
	// Verify regkey to protect the shell endpoint
	regkey := r.Header.Get("regkey")
	if regkey == "" {
		regkey = r.URL.Query().Get("regkey")
	}
	if regkey != utils.Regkey() {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ws, err := shellUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade to websocket:", err)
		return
	}
	defer ws.Close()

	// Try bash first, fallback to sh
	cmdPath := "bash"
	if _, err := os.Stat("/bin/bash"); os.IsNotExist(err) {
		cmdPath = "sh"
	}

	cmd := exec.Command(cmdPath)
	cmd.Env = append(os.Environ(), "TERM=xterm")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Println("Failed to start pty:", err)
		return
	}
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		cmd.Wait()
	}()

	// Copy from PTY to WebSocket
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// Copy from WebSocket to PTY
	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if _, err := ptmx.Write(p); err != nil {
			break
		}
	}
}
