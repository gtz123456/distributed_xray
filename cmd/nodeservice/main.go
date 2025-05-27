package main

import (
	"context"
	"fmt"
	"go-distributed/log"
	"go-distributed/node"
	"go-distributed/registry"
	"go-distributed/service"
	"go-distributed/utils"
	stlog "log"
	"os"
	"time"

	"math/rand"
)

/* node service will manage the xray core */

func main() {
	utils.LoadEnv()

	host, err := utils.GetHostIP()
	if err != nil {
		stlog.Fatalln("Error getting host IP:", err)
	}
	port := os.Getenv("Node_Port")
	if port == "" {
		port = "80"
	}

	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)
	fmt.Println("Service address: ", serviceAddress)

	publicIP, err := utils.GetPublicIP()

	if err != nil {
		stlog.Fatalln("Error getting public IP:", err)
	}

	r := registry.Registration{
		ServiceName:      registry.NodeService,
		ServiceURL:       serviceAddress,
		PublicIP:         publicIP,
		RequiredServices: []registry.ServiceName{registry.LogService, registry.WebService},
		ServiceUpdateURL: serviceAddress + "/services",
	}

	ctx, err := service.Start(context.Background(), host, port, r, node.RegisterHandlers)
	if err != nil {
		stlog.Fatalln(err)
	}

	var logProviders []registry.Registration

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
	log.SetClientLogger(logProvider.ServiceURL, r.ServiceName)

	// get config from web service
	var WebProviders []registry.Registration
	for {
		WebProviders, err = registry.GetProviders(registry.WebService)

		if err != nil {
			stlog.Println("Error getting web service:" + err.Error() + ". Retrying in 3 seconds")
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	fmt.Printf("Web service found at %s\n", WebProviders)

	// WebProvider := WebProviders[0]

	/*resp, err := http.Get(fmt.Sprintf("%s/realitykey", WebProvider))
	if err != nil {
		stlog.Println("Error getting reality key from web service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		stlog.Println("Error getting reality key from web service: %v", resp.Status)
	}

	realitykey := make([]byte, 64)
	_, err = resp.Body.Read(realitykey)
	if err != nil {
		stlog.Println("Error reading reality key from web service: %v", err)
	}

	stlog.Println("Reality key obtained from web service: ", string(realitykey))*/

	REALITY_PRIKEY := os.Getenv("REALITY_PRIKEY")

	utils.ConfigXray(string(REALITY_PRIKEY))

	err = utils.LaunchXray()
	if err != nil {
		stlog.Fatalln("Error launching xray:", err)
	}
	fmt.Println("Xray launched")
	<-ctx.Done()
}
