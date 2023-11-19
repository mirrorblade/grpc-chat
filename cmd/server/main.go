package main

import (
	"context"
	"flag"
	"grpctest/internal/service"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

var (
	host           string
	productionMode bool
)

func init() {
	flag.StringVar(&host, "h", ":1234", "the server's host")
	flag.BoolVar(&productionMode, "p", false, "production mode")
	flag.Parse()
}

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	if productionMode {
		logger, err = zap.NewProduction()
		if err != nil {
			panic(err)
		}
	}

	chatServer := service.NewChatServer(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ch
		close(ch)
		cancel()
	}()

	if err := chatServer.Run(ctx, host); err != nil {
		panic(err)
	}
}
