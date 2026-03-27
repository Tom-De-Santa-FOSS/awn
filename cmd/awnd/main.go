package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tom/awn"
	"github.com/tom/awn/awtreestrategy"
	"github.com/tom/awn/internal/rpc"
	"github.com/tom/awn/internal/transport"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7600", "listen address")
	flag.Parse()

	token := os.Getenv("AWN_TOKEN")

	driver := awn.NewDriver()
	handler := rpc.NewHandler(driver, awtreestrategy.New())
	server := transport.NewServer(handler, *addr, token)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Println("shutting down...")
		driver.CloseAll()
		os.Exit(0)
	}()

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
