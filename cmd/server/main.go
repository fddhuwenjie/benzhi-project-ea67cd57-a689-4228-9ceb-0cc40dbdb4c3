package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"map-registration-gate/internal/app"
	"map-registration-gate/internal/store"
	webui "map-registration-gate/internal/web"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

func main() {
	if e := run(); e != nil {
		log.Printf("服务退出: %v", e)
		os.Exit(1)
	}
}
func run() error {
	addrFlag := flag.String("addr", defaultAddress, "回环监听地址")
	dataFlag := flag.String("data", "data/registration.json", "持久化文件")
	selftest := flag.Bool("selftest", false, "执行真实 HTTP 自检后退出")
	flag.Parse()
	addr, e := configuredAddress(*addrFlag)
	if e != nil {
		return e
	}
	dataPath := *dataFlag
	cleanup := func() {}
	if *selftest {
		dir, e := os.MkdirTemp("", "map-registration-selftest-")
		if e != nil {
			return e
		}
		cleanup = func() { _ = os.RemoveAll(dir) }
		defer cleanup()
		dataPath = filepath.Join(dir, "state.json")
	}
	st, e := store.Open(dataPath)
	if e != nil {
		return fmt.Errorf("打开存储: %w", e)
	}
	svc := app.New(st)
	httpServer := &http.Server{Handler: webui.New(svc).Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	ln, e := net.Listen("tcp", addr)
	if e != nil {
		return fmt.Errorf("监听 %s: %w", addr, e)
	}
	if *selftest {
		return runSelftest(httpServer, ln)
	}
	errch := make(chan error, 1)
	go func() { errch <- httpServer.Serve(ln) }()
	log.Printf("配准审定台已监听 http://%s", addr)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-signals:
		log.Printf("收到信号 %s", sig)
	case e := <-errch:
		if e != nil && e != http.ErrServerClosed {
			return e
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}
