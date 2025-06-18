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

	host, err := utils.GetPublicIP()
	if err != nil {
		stlog.Fatalln("Error getting host IP:", err)
	}
	port := os.Getenv("Log_Port")
	if port == "" {
		port = "80"
	}

	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)

	r := registry.Registration{
		ServiceName:      registry.LogService,
		ServiceURL:       serviceAddress,
		RequiredServices: make([]registry.ServiceName, 0),
		ServiceUpdateURL: serviceAddress + "/service",
	}

	ctx, err := service.Start(context.Background(), host, port, r, log.RegisterHandlers)
	if err != nil {
		stlog.Fatalln(err)
	}
	<-ctx.Done()

	fmt.Println("Log service shut down")
}
