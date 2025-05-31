/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCreateCoreEngineLoggerAvailability(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	r := CreateCoreEngine(logger)
	defer logger.Sync()
	r.GET("/ping", func(c *gin.Context) {
		CtxLogS(c).Infow("Retrieved sugared zap logger", "url", c.Request.URL)
		CtxLog(c).Info("Retrieved zap logger", zap.String("url", c.Request.URL.String()))
		c.String(200, "pong")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("Wrong status code: %d", w.Code)
	}
	if w.Body.String() != "pong" {
		t.Errorf("Wrong body: %s", w.Body.String())
	}
}

func TestCreateCoreEngineErrorReporting(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	r := CreateCoreEngine(logger)
	defer logger.Sync()
	r.GET("/ping", func(c *gin.Context) {
		CtxLogS(c).Errorw("test error 1", "url", c.Request.URL.String())
		CtxLogS(c).Errorw("test error 2", "url", c.Request.URL.String())
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "test error"})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("Wrong status code: %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("Wrong content type: %s", w.Header().Get("Content-Type"))
	}
}

func TestCreateCoreEnginePanicRecovery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	r := CreateCoreEngine(logger)
	defer logger.Sync()
	r.GET("/ping", func(c *gin.Context) {
		CtxLogS(c).Errorw("test error 1", "url", c.Request.URL.String())
		CtxLogS(c).Errorw("test error 2", "url", c.Request.URL.String())
		CtxLogS(c).Panicw("test panic 1", "url", c.Request.URL.String())
		panic("test panic 1")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("Wrong status code: %d", w.Code)
	}
	if len(w.Body.String()) != 0 {
		t.Errorf("Wrong body: %s", w.Body.String())
	}
}

func TestCreateTestContextLoggerAndRequestAvailability(t *testing.T) {
	c, _ := CreateTestContext()
	CtxLogS(c).Infow("Retrieved sugared zap logger", "url", c.Request.URL.String())
	CtxLog(c).Info("Retrieved zap logger", zap.String("url", c.Request.URL.String()))
	if c.Request.Context() != context.Background() {
		t.Errorf("request context is not background context")
	}
}
