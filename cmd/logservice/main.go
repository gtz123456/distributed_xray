package main

import (
	"context"
	"fmt"
	"go-distributed/log"
	"go-distributed/registry"
	"go-distributed/service"
	stlog "log"
)

func main() {
	log.Run("distributed.log")
	host, port := "localhost", "4000"
	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)
	// TODO: make host and port configurable

	r := registry.Registration{
		ServiceName:      "LogService",
		ServiceURL:       serviceAddress,
		RequiredServices: make([]registry.ServiceName, 0),
		ServiceUpdateURL: serviceAddress + "/services",
	}

	ctx, err := service.Start(context.Background(), host, port, r, log.RegisterHundlers)
	if err != nil {
		stlog.Fatalln(err)
	}
	<-ctx.Done()

	fmt.Println("Log service shut down")
}
