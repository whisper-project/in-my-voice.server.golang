/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package lifecycle

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
)

// Startup takes a configured router and runs a server instance with it as handler.
// The instance is configured so that it can be exited cleanly.
//
// This code based on [this example](https://github.com/gin-gonic/examples/blob/master/graceful-shutdown/graceful-shutdown/notify-with-context/server.go)
func Startup(router *gin.Engine, hostPort string) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the speech monitors
	stopMonitors := startMonitors()

	// Run the server in a goroutine so that this instance survives it
	running := true
	srv := &http.Server{Addr: hostPort, Handler: router}
	go func() {
		log.Printf("Server listening on %s...", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sLog().Error("http server crashed", zap.Error(err))
		}
		running = false
	}()

	// Listen for the interrupt signal, and restore default behavior
	<-ctx.Done()
	stop()
	sLog().Info("interrupt received")

	// Shutdown the http server instance cleanly.
	// If the server is still running, we give it 15 seconds to stop cleanly.
	ctx, cancel := context.WithTimeout(sCtx(), 15*time.Second)
	storage.ServerContext = ctx
	defer cancel()
	if running {
		go func() {
			if err := srv.Shutdown(ctx); err != nil {
				sLog().Error("http server terminated with error", zap.Error(err))
				return
			}
			sLog().Info("http server gracefully stopped")
		}()
	}

	// Stop the monitors and then force close everything
	sLog().Info("Stopping monitors...")
	if stopMonitors() != nil {
		sLog().Info("Monitors failed to stop cleanly")
	} else {
		sLog().Info("Monitors stopped cleanly")
	}
	sLog().Info("server instance shutdown complete")
}

func CreateEngine() (*gin.Engine, error) {
	var logger *zap.Logger
	var err error
	if platform.GetConfig().Name == "production" {
		gin.SetMode(gin.ReleaseMode)
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		return nil, err
	}
	defer logger.Sync()
	engine := middleware.CreateCoreEngine(logger)
	err = engine.SetTrustedProxies(nil)
	if err != nil {
		return nil, err
	}
	storage.ServerLogger = logger
	storage.ServerContext = context.Background()
	return engine, nil
}

func sLog() *zap.Logger {
	return storage.ServerLogger
}

func sCtx() context.Context {
	return storage.ServerContext
}
