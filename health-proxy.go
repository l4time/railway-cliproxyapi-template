package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "0.0.0.0:8080", "public listen address")
	upstream := flag.String("upstream", "127.0.0.1:8317", "private upstream address")
	binary := flag.String("binary", "/CLIProxyAPI/CLIProxyAPI", "upstream binary")
	config := flag.String("config", "/data/state/config.yaml", "upstream config")
	flag.Parse()

	cmd := exec.Command(*binary, "-config", *config)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("upstream start failed: %v", err)
	}

	target, _ := url.Parse("http://" + *upstream)
	proxy := httputil.NewSingleHostReverseProxy(target)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		c, err := net.DialTimeout("tcp", *upstream, 250*time.Millisecond)
		if err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		_ = c.Close()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/", proxy)

	server := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	errs := make(chan error, 2)
	go func() { errs <- cmd.Wait() }()
	go func() { errs <- server.ListenAndServe() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	unexpected := false
	select {
	case sig := <-signals:
		_ = cmd.Process.Signal(sig)
	case err := <-errs:
		unexpected = true
		if err != nil && err != http.ErrServerClosed {
			log.Printf("process stopped: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	if cmd.ProcessState == nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	fmt.Println("shutdown complete")
	if unexpected {
		os.Exit(1)
	}
}
