/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
)

func AnomalyHandler(c *gin.Context) {
	clientId := c.GetHeader("X-Client-Id")
	profileId := c.GetHeader("X-Profile-Id")
	clientType := c.GetHeader("X-Client-Type")
	message := c.PostForm("message")
	middleware.CtxLog(c).Info("Anomaly reported",
		zap.String("clientId", clientId), zap.String("clientType", clientType),
		zap.String("profileId", profileId), zap.String("message", message))
	c.Status(http.StatusNoContent)
}

func LaunchHandler(c *gin.Context) {
	clientType := c.GetHeader("X-Client-Type")
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	storage.ObserveClientLaunch(clientType, clientId, profileId)
	// make sure the client knows whether they're enrolled in the study
	studyId, _, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	studyName := "none"
	if studyId != "" {
		study, err := storage.GetStudy(studyId)
		if err != nil || study == nil {
			if study == nil {
				middleware.CtxLog(c).Warn("enrolled study has gone missing", zap.String("studyId", studyId))
			}
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
			return
		}
		studyName = study.Name
	}
	c.Header("X-Study-Membership-Update", studyName)
	// make sure any other update annotation has been removed,
	// because clients update everything at launch
	c.Header("X-Speech-Settings-Update", "")
	c.Header("X-Favorites-Update", "")
	middleware.CtxLog(c).Info("Launch received",
		zap.String("clientType", clientType), zap.String("clientId", clientId),
		zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
	return
}

func ForegroundHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	storage.ObserveClientActive(clientId, profileId)
	middleware.CtxLog(c).Info("Foreground received",
		zap.String("clientId", clientId), zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
}

func BackgroundHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	storage.ObserveClientActive(clientId, profileId)
	middleware.CtxLog(c).Info("Background received",
		zap.String("clientId", clientId), zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
}

func ShutdownHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	storage.ObserveClientShutdown(clientId, profileId)
	middleware.CtxLog(c).Info("Shutdown received",
		zap.String("clientId", clientId), zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
}

func ValidateRequest(c *gin.Context) (clientId, profileId string, ok bool) {
	clientId = c.GetHeader("X-Client-Id")
	profileId = c.GetHeader("X-Profile-Id")
	if uuid.Validate(clientId) != nil || uuid.Validate(profileId) != nil {
		middleware.CtxLog(c).Info("Invalid client or profile ID",
			zap.String("clientId", clientId), zap.String("profileId", profileId))
		c.Header("X-Message", "**Something went wrong.**\nPlease reinstall the app and try again.")
		c.AbortWithStatusJSON(400, gin.H{"status": "error", "error": "invalid client or profile id"})
		return "", "", false
	}
	AnnotateResponse(c, clientId, profileId)
	return clientId, profileId, true
}

func AnnotateResponse(c *gin.Context, clientId, profileId string) {
	needsNotification, _ := storage.ProfileClientSpeechNeedsNotification(profileId, clientId)
	if needsNotification {
		c.Header("X-Speech-Settings-Update", "YES")
		_ = storage.ProfileClientSpeechWasNotified(profileId, clientId)
	}
	needsNotification, _ = storage.ProfileClientFavoritesNeedsNotification(profileId, clientId)
	if needsNotification {
		c.Header("X-Favorites-Update", "YES")
		_ = storage.ProfileClientFavoritesWasNotified(profileId, clientId)
	}
	needsNotification, _ = storage.ProfileClientUsageNeedsNotification(profileId, clientId)
	if needsNotification {
		c.Header("X-Usage-Update", "YES")
		_ = storage.ProfileClientUsageWasNotified(profileId, clientId)
	}
}
