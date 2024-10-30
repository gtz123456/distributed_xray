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
	"net/http"
	"os"
	"time"

	"math/rand"
)

/* node service will manage the xray core */

func main() {
	utils.LoadEnv()

	host := utils.GetHostIP()
	port := os.Getenv("Node_Port")
	if port == "" {
		port = "80"
	}

	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)

	r := registry.Registration{
		ServiceName:      registry.NodeService,
		ServiceURL:       serviceAddress,
		RequiredServices: []registry.ServiceName{registry.LogService, registry.WebService},
		ServiceUpdateURL: serviceAddress + "/services",
	}

	ctx, err := service.Start(context.Background(), host, port, r, shell.RegisterHandlers)
	if err != nil {
		stlog.Fatalln(err)
	}

	var logProviders []string

	for {
		logProviders, err = registry.GetProviders(registry.LogService)

		if err != nil {
			stlog.Println("Error getting log service:" + err.Error() + ". Retrying in 3 seconds")
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	fmt.Printf("Logging service found at %s\n", logProviders)
	// select a logger provider randomly
	// TODO: Select logger based on lattency??
	logProvider := logProviders[rand.Intn(len(logProviders))]
	log.SetClientLogger(logProvider, r.ServiceName)

	// get config from web service
	var WebProviders []string
	for {
		WebProviders, err = registry.GetProviders(registry.WebService)

		if err != nil {
			stlog.Println("Error getting log service:" + err.Error() + ". Retrying in 3 seconds")
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	fmt.Printf("Web service found at %s\n", WebProviders)

	WebProvider := WebProviders[0]

	resp, err := http.Get(fmt.Sprintf("%s/realitykey", WebProvider))
	if err != nil {
		stlog.Println("Error getting reality key from web service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		stlog.Fatalf("Error getting reality key from web service: %v", resp.Status)
	}

	realitykey := make([]byte, 64)
	_, err = resp.Body.Read(realitykey)
	if err != nil {
		stlog.Fatalf("Error reading reality key from web service: %v", err)
	}

	utils.ConfigXray(string(realitykey))

	utils.LaunchXray()
	fmt.Println("Xray launched")
	<-ctx.Done()
}
