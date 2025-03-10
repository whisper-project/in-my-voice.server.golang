/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
)

func sLog() *zap.Logger {
	return storage.ServerLogger
}

func sCtx() context.Context {
	return storage.ServerContext
}

func StatusHandler(c *gin.Context) {
	sLog().Info("Returning status OK",
		zap.String("serverId", storage.ServerId))
	c.JSON(200, gin.H{
		"status":   "ok",
		"serverId": storage.ServerId,
	})
}
