/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func LaunchHandler(c *gin.Context) {
	clientType := c.GetHeader("X-Client-Type")
	clientId := c.GetHeader("X-Client-Id")
	profileId := c.GetHeader("X-Profile-Id")
	storage.ObserveClientLaunch(clientType, clientId, profileId)
	middleware.CtxLog(c).Info("Launch received",
		zap.String("clientType", clientType), zap.String("clientId", clientId),
		zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
	return
}

func ShutdownHandler(c *gin.Context) {
	clientId := c.GetHeader("X-Client-Id")
	storage.ObserveClientShutdown(clientId)
	c.Status(http.StatusNoContent)
}
