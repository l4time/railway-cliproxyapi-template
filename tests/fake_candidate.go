package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var version = "v7.2.142"

const fixtureManagementKey = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

func configValue(data []byte, prefix string) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"`)
		}
	}
	return ""
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("CLIProxyAPI Version: %s, Commit: fixture, BuiltAt: fixture\n", version)
		return
	}
	configPath := flag.String("config", "", "config")
	flag.Parse()
	data, err := os.ReadFile(*configPath)
	if err != nil {
		os.Exit(2)
	}
	port, err := strconv.Atoi(configValue(data, "port:"))
	if err != nil {
		os.Exit(2)
	}
	proxyKey := configValue(data, "- ")
	managementKey := configValue(data, "secret-key:")
	if port == 8317 {
		managementKey = fixtureManagementKey
	}
	ui, err := os.ReadFile(os.Getenv("MANAGEMENT_STATIC_PATH"))
	if err != nil {
		os.Exit(2)
	}
	if fmt.Sprintf("%x", sha256.Sum256(ui)) != "e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4" {
		os.Exit(2)
	}
	mode := os.Getenv("CLIPROXY_FAKE_CANDIDATE_MODE")
	authorized := func(r *http.Request, key string) bool {
		return r.Header.Get("Authorization") == "Bearer "+key
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if mode == "bad-probe-auth" && port != 8317 {
			w.WriteHeader(http.StatusOK)
			return
		}
		if !authorized(r, proxyKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	})
	mux.HandleFunc("/v0/management/config", func(w http.ResponseWriter, r *http.Request) {
		if mode == "bad-live" && port == 8317 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if !authorized(r, managementKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"fixture": true}})
	})
	mux.HandleFunc("/management.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(ui)
	})
	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
		<-signals
		_ = server.Close()
	}()
	if mode == "crash-live" && port == 8317 {
		go func() {
			for i := 0; i < 1000000; i++ {
			}
			os.Exit(23)
		}()
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		os.Exit(3)
	}
}
