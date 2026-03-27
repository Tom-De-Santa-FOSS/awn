package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tom/awn/internal/rpc"
	"github.com/tom/awn/internal/session"
	"github.com/tom/awn/internal/transport"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7600", "listen address")
	flag.Parse()

	token := os.Getenv("AWN_TOKEN")

	mgr := session.NewManager()
	handler := rpc.NewHandler(mgr)
	server := transport.NewServer(handler, *addr, token)

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Println("shutting down...")
		os.Exit(0)
	}()

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
