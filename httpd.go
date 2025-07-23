package main

import (
	"context"
	"net/http"

	"fmt"
	"log"
	"strings"
	"sync"

	"rpi-bot/messaging"
	"time"
)

func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if token != "" && auth != "Token "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func setupMux(cfg *Config, commandHandler *httpCommandHandler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	cmdHandler := authMiddleware(cfg.Httpd.AuthToken, http.HandlerFunc(commandHandler.ServeHTTP))
	mux.Handle("/cmd/", cmdHandler)

	return mux
}

func HttpServer(ctx context.Context, cfg *Config, executor commandExecutor, wg *sync.WaitGroup) {
	defer wg.Done()

	authToken, _ := GetSecret("HTTP_TOKEN_AUTH", cfg.Httpd.AuthToken)
	commandHandler := &httpCommandHandler{
		commands:  cfg.Commands,
		authToken: authToken,
		executor:  executor,
	}
	httpSrv := &http.Server{
		Addr:    cfg.Httpd.Addr,
		Handler: setupMux(cfg, commandHandler),
	}
	// Gracefully shut down HTTP server on context cancel
	go func() {
		<-ctx.Done()
		log.Println("Shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server Shutdown error: %v", err)
		}
	}()

	// httpSrv.ListenAndServe returns err when server stops or fails

	log.Println("Starting httpd server")
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// If it's ErrServerClosed, that means Shutdown() was called.
		log.Printf("HTTP server ListenAndServe(): %v", err)
	}

}

type httpCommandHandler struct {
	commands  map[string]Command
	authToken string
	executor  commandExecutor
}

func (h *httpCommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cmdName := strings.TrimPrefix(r.URL.Path, "/cmd/")
	if cmdName == "" {
		http.Error(w, "no command specified", http.StatusBadRequest)
		return
	}

	cmdDef, ok := h.commands[cmdName]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown command %q", cmdName), http.StatusNotFound)
		return
	}

	query := r.URL.Query()
	values := make([]string, 0, len(cmdDef.Args))
	for _, argKey := range cmdDef.Args {
		v := query.Get(argKey)
		if v == "" {
			http.Error(
				w,
				fmt.Sprintf("missing required query parameter %q", argKey),
				http.StatusBadRequest,
			)
			return
		}
		values = append(values, v)
	}
	msg := messaging.Message{Command: cmdName, Args: values}
	fullCmd, err := createCommand(cmdDef, msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	output, err := h.executor.execCommand(fullCmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	w.Header().Set("Content-Type", "text/plain")
	_, err = fmt.Fprint(w, output)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
