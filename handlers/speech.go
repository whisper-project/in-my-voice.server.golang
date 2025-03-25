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

func ElevenSpeechSettingsGetHandler(c *gin.Context) {
	_, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	s, err := storage.GetSpeechSettings(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if s == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": "profile has no speech settings"})
		return
	}
	c.JSON(http.StatusOK, json.RawMessage(s.Settings))
}

func ElevenSpeechSettingsPutHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		middleware.CtxLog(c).Error("failed to read settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to read request body"})
		return
	}
	changed, err := storage.UpdateSpeechSettings(profileId, string(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if changed {
		if err := storage.ProfileClientSpeechDidUpdate(profileId, clientId); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func ElevenSpeechFailureHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	var body map[string]any
	if err := c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "malformed body"})
		return
	}
	action, aOk := body["action"].(string)
	code, cOk := body["code"].(float64)
	if !aOk || !cOk {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid code or action"})
		return
	}
	reason := "ElevenLabs call failed"
	if code == 401 {
		reason += ": invalid_api_key"
	} else {
		if response, ok := body["response"].(map[string]any); ok {
			if detail, ok := response["detail"].(map[string]any); ok {
				if status, ok := detail["status"].(string); ok && status != "" {
					reason += ": " + status
				} else if message, ok := detail["message"].(string); ok && message != "" {
					reason += ": " + message
				}
			}
		} else if response, ok := body["response"].(string); ok {
			reason += ": " + response
		}
	}
	middleware.CtxLog(c).Info(reason,
		zap.String("clientId", clientId), zap.String("profileId", profileId),
		zap.String("action", action), zap.Int("responseCode", int(code)),
		zap.Any("response", body["response"]))
	c.Status(http.StatusNoContent)
	return
}
