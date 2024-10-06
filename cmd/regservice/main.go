package main

import (
	"context"
	"fmt"
	"go-distributed/registry"
	"go-distributed/registry/heartbeat"
	"go-distributed/utils"
	"log"
	"net/http"
)

func main() {
	utils.LoadEnv()

	http.Handle("/services", &registry.RegistryService{})
	http.Handle("/heartbeat/", heartbeat.NewHeartBeatServer())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var srv http.Server
	srv.Addr = "localhost" + registry.ServerPort

	go func() {
		log.Println(srv.ListenAndServe())
		cancel()
	}()

	go func() {
		fmt.Println("Registry service started. Press any key to stop.")
		var s string
		fmt.Scanln(&s)
		srv.Shutdown(context.Background())
		cancel()
	}()

	<-ctx.Done()

	fmt.Println("Shutting down registry service")
}
