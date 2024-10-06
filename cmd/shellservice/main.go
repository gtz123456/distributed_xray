package main

import (
	"context"
	"fmt"
	"go-distributed/log"
	"go-distributed/registry"
	"go-distributed/service"
	"go-distributed/shell"
	"go-distributed/utils"
	stlog "log"

	"math/rand"
)

func main() {
	utils.LoadEnv()

	host, port := "localhost", "4001"

	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)

	r := registry.Registration{
		ServiceName:      registry.ShellService,
		ServiceURL:       serviceAddress,
		RequiredServices: []registry.ServiceName{registry.LogService},
		ServiceUpdateURL: serviceAddress + "/services",
	}

	ctx, err := service.Start(context.Background(), host, port, r, shell.RegisterHandlers)
	if err != nil {
		stlog.Fatalln(err)
	}

	logProviders, err := registry.GetProvider(registry.LogService)

	if err != nil {
		stlog.Fatalf("Error getting log service: %v", err)
	}

	fmt.Printf("Logging service found at %s\n", logProviders)
	// select a logger provider randomly
	logProvider := logProviders[rand.Intn(len(logProviders))]
	log.SetClientLogger(logProvider, r.ServiceName)

	<-ctx.Done()
}
