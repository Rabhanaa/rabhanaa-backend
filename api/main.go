package main

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	appctx "rabhana/api/context"
	httproutes "rabhana/api/http"
	"rabhana/lib/logger"
	"rabhana/pkg/config"
)

func main() {
	// Setup logger
	logger.Setup(
		config.GetEnv("LOG_LEVEL", "info"),
		config.GetEnv("LOG_FORMAT", "json"),
	)

	// Load config
	cfg := appctx.LoadAppConfig()

	// Connect to database
	ctx := context.Background()
	appContext, err := appctx.NewAppContext(ctx, cfg)
	if err != nil {
		slog.Error("failed to create app context", "error", err)
		os.Exit(1)
	}
	defer appContext.DB.Close()

	// Setup router
	router := gin.Default()
	httproutes.RegisterRoutes(router, appContext)

	// Start cron jobs
	if appContext.CronService != nil {
		appContext.CronService.Start()
		defer appContext.CronService.Stop()
	}

	// Graceful shutdown
	srv := &stdhttp.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	slog.Info("server started", "port", cfg.ServerPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
