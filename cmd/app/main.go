package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/idkroff/cf-balancer/internal/balancer"
	"github.com/idkroff/cf-balancer/internal/config"
	addRequestHandler "github.com/idkroff/cf-balancer/internal/http-server/handlers/request/add"
	getRequestHandler "github.com/idkroff/cf-balancer/internal/http-server/handlers/request/get"
	mwLogger "github.com/idkroff/cf-balancer/internal/http-server/middleware/logger"
)

func main() {
	config := config.MustLoad()

	log := setupLogger(config.Env)

	log.Info("starting cf-balancer", slog.String("env", config.Env))

	b := balancer.New(config.CFLimits, log)
	b.StartQueueTimers()

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Post("/request", addRequestHandler.New(log, b))
	router.Get("/request/{UUID}", getRequestHandler.New(log, b))

	log.Info("starting server", slog.String("address", config.Address))
	server := &http.Server{
		Addr:         config.Address,
		Handler:      router,
		ReadTimeout:  config.HTTPServer.Timeout,
		WriteTimeout: config.HTTPServer.Timeout,
		IdleTimeout:  config.HTTPServer.IdleTimeout,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Error("failed to start server")
	}

	log.Error("server stopped")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case "local":
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case "dev":
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case "prod":
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
