package main

import (
	"context"
	"fmt"
	"go-distributed/log"
	"go-distributed/registry"
	"go-distributed/service"
	"go-distributed/utils"
	stlog "log"
	"os"
)

func main() {
	log.Run("distributed.log")
	utils.LoadEnv()
	host, port := "localhost", os.Getenv("logport")
	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)
	// TODO: make host and port configurable

	r := registry.Registration{
		ServiceName:      registry.LogService,
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
