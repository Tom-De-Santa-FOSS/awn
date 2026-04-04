package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tom/awn"
	"github.com/tom/awn/awtreestrategy"
	"github.com/tom/awn/internal/rpc"
	"github.com/tom/awn/internal/transport"
	"go.uber.org/zap"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7600", "listen address")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	token := os.Getenv("AWN_TOKEN")
	stateDir := resolveStateDir(os.Getenv("AWN_STATE_DIR"), os.UserCacheDir)

	driver := awn.NewDriver(awn.WithPersistenceDir(stateDir), awn.WithLogger(logger))
	handler := rpc.NewHandler(driver, awtreestrategy.New(), logger)
	server := transport.NewServer(handler, *addr, token, logger)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Info("shutting down")
		driver.CloseAll()
		os.Exit(0)
	}()

	if err := server.ListenAndServe(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

func resolveStateDir(envValue string, userCacheDir func() (string, error)) string {
	if envValue != "" {
		return envValue
	}
	cacheDir, err := userCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, "awn", "sessions")
}
