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
	"os"

	"math/rand"
)

/* shell service is mainly for testing registry client and other utils */

func main() {
	utils.LoadEnv()

	host := utils.GetHostIP()
	port := os.Getenv("Shell_Port")
	if port == "" {
		port = "80"
	}

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

	logProviders, err := registry.GetProviders(registry.LogService)

	if err != nil {
		stlog.Fatalf("Error getting log service: %v", err)
	}

	fmt.Printf("Logging service found at %s\n", logProviders)
	// select a logger provider randomly
	logProvider := logProviders[rand.Intn(len(logProviders))]
	log.SetClientLogger(logProvider, r.ServiceName)

	<-ctx.Done()
}
