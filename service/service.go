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
	ctx = startService(ctx, reg.ServiceName, host, port)

	err := registry.RegisterService(reg)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func startService(ctx context.Context, serviceName registry.ServiceName, host, port string) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	var srv http.Server
	srv.Addr = ":" + port

	go func() {
		log.Println(srv.ListenAndServe())
		err := registry.ShutdownService(fmt.Sprintf("http://%v:%v", host, port))
		if err != nil {
			log.Println(err)
		}
		cancel()
	}()

	go func() {
		fmt.Printf("%v started. Press any key to stop.\n", serviceName)
		var s string
		fmt.Scanln(&s)
		fmt.Println("De-registering...")
		err := registry.ShutdownService(fmt.Sprintf("http://%v:%v", host, port))
		if err != nil {
			log.Println(err)
		}
		fmt.Println("Shutting down server")
		srv.Shutdown(ctx)
		cancel()
	}()

	return ctx
}
