/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
	"io"
	"net/http"
)

func FavoritesGetHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	f, err := storage.GetFavoritesSettings(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// make sure any update annotation has been removed
	c.Header("X-Favorites-Update", "")
	if f != nil {
		middleware.CtxLog(c).Info("successful favorites retrieval",
			zap.String("clientId", clientId), zap.String("profileId", profileId))
		c.JSON(http.StatusOK, json.RawMessage(f.Settings))
		return
	}
	middleware.CtxLog(c).Info("no favorites to retrieve",
		zap.String("clientId", clientId), zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
}

func FavoritesPutHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	// make sure any update annotation has been removed
	c.Header("X-Favorites-Update", "")
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		middleware.CtxLog(c).Error("failed to read settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to read request body"})
		return
	}
	changed, err := storage.UpdateFavoritesSettings(profileId, string(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if changed {
		if err := storage.ProfileClientFavoritesDidUpdate(profileId, clientId); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}
