package service

import (
	"context"
	"fmt"
	"go-distributed/registry"
	"log"
	"net/http"
)

func Start(ctx context.Context, host, port string, reg registry.Registration, registerHundlersFunc func()) (context.Context, error) {
	registerHundlersFunc()
	log.Printf("Starting service %s at %s:%s\n", reg.ServiceName, host, port)
	ctx = startService(ctx, reg.ServiceName, host, port)
	log.Printf("Service %s started at %s:%s\n", reg.ServiceName, host, port)

	err := registry.RegisterService(&reg)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func startService(ctx context.Context, serviceName registry.ServiceName, host, port string) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	var srv http.Server
	srv.Addr = host + ":" + port

	go func() {
		log.Println(srv.ListenAndServe())
		err := registry.ShutdownService(fmt.Sprintf("http://%v:%v", host, port))
		if err != nil {
			log.Println(err)
		}
		cancel()
	}()

	return ctx
}
