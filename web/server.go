package user

import "net/http"

type userHandler struct{}

// use gin instead of http

func RegisterHandlers() {
	// left emply
}

func (uh *userHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// left empty
}
