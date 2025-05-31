/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
)

func StatusHandler(c *gin.Context) {
	middleware.CtxLog(c).Info("Returning status OK",
		zap.String("serverId", storage.ServerId))
	c.JSON(200, gin.H{
		"status":   "ok",
		"serverId": storage.ServerId,
	})
}
