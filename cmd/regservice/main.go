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
	srv.Addr = registry.ServerIP + ":" + registry.ServerPort

	go func() {
		log.Println(srv.ListenAndServe())
		cancel()
	}()

	fmt.Println("Registry service is running on ", srv.Addr)

	<-ctx.Done()

	fmt.Println("Shutting down registry service")
}
