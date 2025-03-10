/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package middleware

import (
	"net/http"
	"net/http/httptest"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreateCoreEngine returns a gin router with zap logging and recovery
func CreateCoreEngine(logger *zap.Logger) *gin.Engine {
	r := gin.New()
	defer logger.Sync()
	r.Use(ginzap.Ginzap(logger, time.RFC3339, false))
	r.Use(ginzap.RecoveryWithZap(logger, false))
	r.Use(AddCtxLoggers(logger))
	return r
}

// CreateTestContext returns a gin Context and engine for testing.
//
// Unlike a raw Gin-prepared test context, this one has a request.
func CreateTestContext() (*gin.Context, *gin.Engine) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	c, e := gin.CreateTestContext(w)
	c.Request = req
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	e.Use(ginzap.Ginzap(logger, time.RFC3339, false))
	e.Use(ginzap.RecoveryWithZap(logger, false))
	sugar := logger.Sugar()
	c.Set("sweet", sugar)
	c.Set("unsweet", logger)
	return c, e
}
