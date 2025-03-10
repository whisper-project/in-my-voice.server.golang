/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AddCtxLoggers middleware makes the logger available to handlers.
//
// Both sugared and non-sugared versions are available.
func AddCtxLoggers(logger *zap.Logger) gin.HandlerFunc {
	sugar := logger.Sugar()
	return func(c *gin.Context) {
		c.Set("sweet", sugar)
		c.Set("unsweet", logger)
		c.Next()
	}
}

func CtxLog(ctx *gin.Context) *zap.Logger {
	val, ok := ctx.Get("unsweet")
	if !ok {
		panic(fmt.Sprintf("No unsweet logger found on context: %#v", ctx))
	}
	logger, ok := val.(*zap.Logger)
	if !ok {
		panic(fmt.Sprintf("Logger is not a zap logger: %#v", val))
	}
	return logger
}

func CtxLogS(ctx *gin.Context) *zap.SugaredLogger {
	val, ok := ctx.Get("sweet")
	if !ok {
		panic(fmt.Sprintf("No unsweet logger found on context: %#v", ctx))
	}
	logger, ok := val.(*zap.SugaredLogger)
	if !ok {
		panic(fmt.Sprintf("Logger is not a zap sugared logger: %#v", val))
	}
	return logger
}
