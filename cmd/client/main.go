package main

import (
	"bufio"
	"flag"
	"os"
	"os/signal"
	"syscall"
)

var (
	host   string
	myName string
)

func init() {
	flag.StringVar(&host, "h", ":1234", "the server's host")
	flag.StringVar(&myName, "n", "user1", "your name")
	flag.Parse()
}

func main() {
	chatClient := newClient(bufio.NewScanner(os.Stdin), myName)

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ch

		chatClient.Close()
		os.Exit(0)
	}()

	if err := chatClient.Run(host); err != nil {
		panic(err)
	}

}
