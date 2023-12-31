package main

import (
	"log"
	"net/http"
	"syscall"

	"github.com/shinhagunn/websocket/config"
	"github.com/shinhagunn/websocket/handlers"
	"github.com/shinhagunn/websocket/pkg/routing"
)

const numberOfWorker = 5

func main() {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("pprof failed: %v", err)
		}
	}()

	config, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	epoll, err := routing.NewEpoll(config)
	if err != nil {
		panic(err)
	}

	go epoll.Read()

	for i := 0; i < numberOfWorker; i++ {
		go epoll.Write()
	}

	if err := handlers.SetupRoutes(config, epoll); err != nil {
		panic(err)
	}
}
