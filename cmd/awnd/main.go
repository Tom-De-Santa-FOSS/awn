package main

import (
	"flag"
	"fmt"
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

var version = "dev"
var commit = "none"

// defaultSocketPath returns ~/.awn/daemon.sock.
func defaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".awn", "daemon.sock")
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Printf("awnd v%s (%s)\n", version, commit)
			os.Exit(0)
		}
	}
	tcpMode := flag.Bool("tcp", false, "listen on TCP instead of Unix socket (requires AWN_TOKEN)")
	addr := flag.String("addr", "127.0.0.1:7600", "TCP listen address (only used with --tcp)")
	socketPath := flag.String("socket", "", "Unix socket path (default ~/.awn/daemon.sock)")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	token := os.Getenv("AWN_TOKEN")
	stateDir := resolveStateDir(os.Getenv("AWN_STATE_DIR"), os.UserCacheDir)

	// Override addr from env if set.
	if envAddr := os.Getenv("AWN_ADDR"); envAddr != "" && *tcpMode {
		*addr = envAddr
	}

	driver := awn.NewDriver(awn.WithPersistenceDir(stateDir), awn.WithLogger(logger))
	handler := rpc.NewHandler(driver, awtreestrategy.New(), logger)
	server := transport.NewServer(handler, *addr, token, logger)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Info("shutting down")
		server.Close()
		driver.CloseAll()
		os.Exit(0)
	}()

	if *tcpMode {
		if token == "" {
			fmt.Fprintln(os.Stderr, "error: --tcp requires AWN_TOKEN to be set. TCP mode exposes the daemon to the network.")
			fmt.Fprintln(os.Stderr, "Set AWN_TOKEN=<secret> or remove --tcp to use the default Unix socket.")
			os.Exit(1)
		}
		logger.Info("starting in TCP mode", zap.String("addr", *addr))
		if err := server.ListenAndServe(); err != nil {
			logger.Fatal("server failed", zap.Error(err))
		}
	} else {
		sock := *socketPath
		if sock == "" {
			if envSock := os.Getenv("AWN_SOCKET"); envSock != "" {
				sock = envSock
			} else {
				sock = defaultSocketPath()
			}
		}
		// Ensure parent directory exists.
		if dir := filepath.Dir(sock); dir != "" {
			os.MkdirAll(dir, 0o755) //nolint:errcheck
		}
		logger.Info("starting in Unix socket mode", zap.String("socket", sock))
		if err := server.ListenAndServeUnix(sock); err != nil {
			logger.Fatal("server failed", zap.Error(err))
		}
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
