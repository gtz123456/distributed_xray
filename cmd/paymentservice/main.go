package main

import (
	"context"
	"fmt"
	"go-distributed/payment/db"
	"go-distributed/payment/order"
	"go-distributed/registry"
	"go-distributed/service"
	"go-distributed/utils"
	stlog "log"
	"os"
)

func init() {
	utils.LoadEnv()
	db.Connect()
	db.Sync()
}

func main() {
	host, err := utils.GetPublicIP()
	if err != nil {
		stlog.Fatalln("Error getting host IP:", err)
	}
	port := os.Getenv("Payment_Port")
	if port == "" {
		port = "80"
	}

	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)

	r := registry.Registration{
		ServiceName:      registry.PaymentService,
		ServiceURL:       serviceAddress,
		RequiredServices: []registry.ServiceName{},
		ServiceUpdateURL: serviceAddress + "/services",
	}

	ctx, err := service.Start(context.Background(), "localhost", port, r, order.RegisterHandlers)
	if err != nil {
		stlog.Fatalln(err)
	}

	<-ctx.Done()
}
