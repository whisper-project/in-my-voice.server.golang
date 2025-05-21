/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
)

func ElevenSpeechSettingsGetHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	s, err := storage.GetSpeechSettings(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// make sure any update annotation has been removed
	c.Header("X-Speech-Settings-Update", "")
	if s != nil {
		middleware.CtxLog(c).Info("successful speech settings retrieval",
			zap.String("clientId", clientId), zap.String("profileId", profileId))
		c.JSON(http.StatusOK, json.RawMessage(s.Settings))
		return
	}
	middleware.CtxLog(c).Info("no speech settings to retrieve",
		zap.String("clientId", clientId), zap.String("profileId", profileId))
	c.Status(http.StatusNoContent)
}

func ElevenSpeechSettingsPostHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		middleware.CtxLog(c).Error("failed to read settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to read the request body"})
		return
	}
	apiKey, voiceId, voiceName, ok := services.ElevenParseSettings(string(body))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid settings"})
		return
	}
	if apiKey == "" {
		// user wants to delete their voice settings, oblige them
		if err := storage.DeleteSpeechSettings(profileId); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
			return
		}
		if err := storage.ProfileClientSpeechDidUpdate(profileId, clientId); err != nil {
			middleware.CtxLog(c).Info("ignoring update notifications error",
				zap.String("profileId", profileId), zap.Error(err))
		}
		c.Header("X-Speech-Settings-Update", "true")
		c.Status(http.StatusNoContent)
		return
	}
	if ok, err := services.ElevenValidateApiKey(apiKey); err != nil {
		middleware.CtxLog(c).Error("network failure validating the API key", zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "error": "Network error reaching ElevenLabs"})
		return
	} else if !ok {
		middleware.CtxLog(c).Info("invalid API key", zap.String("apiKey", apiKey),
			zap.String("clientId", clientId), zap.String("profileId", profileId))
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "error": "invalid API key"})
		return
	}
	if voiceId == "" {
		voices, err := services.ElevenFetchVoices(apiKey)
		if err != nil {
			middleware.CtxLog(c).Error("network failure fetching voices", zap.Error(err))
			c.JSON(http.StatusBadGateway, gin.H{"status": "error", "error": "Network error reaching ElevenLabs"})
			return
		}
		middleware.CtxLog(c).Info("apiKey OK, returning voices", zap.Int64("voiceCount", int64(len(voices))),
			zap.String("clientId", clientId), zap.String("profileId", profileId))
		c.JSON(http.StatusOK, voices)
		return
	}
	name, ok, err := services.ElevenValidateVoiceId(apiKey, voiceId)
	if err != nil {
		middleware.CtxLog(c).Error("network failure validating voice ID", zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "error": "Network error reaching ElevenLabs"})
		return
	}
	if !ok {
		middleware.CtxLog(c).Info("invalid voice ID", zap.String("voiceId", voiceId),
			zap.String("clientId", clientId), zap.String("profileId", profileId))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "invalid voice ID"})
		return
	}
	if name != voiceName {
		middleware.CtxLog(c).Info("the uploaded voice name does not match voice ID, fixing it",
			zap.String("actual name", voiceId), zap.String("uploaded name", voiceName),
		)
	}
	// validation complete: update the voice settings
	changed, err := storage.UpdateSpeechSettings(profileId, apiKey, voiceId, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if changed {
		if err := storage.ProfileClientSpeechDidUpdate(profileId, clientId); err != nil {
			middleware.CtxLog(c).Info("ignoring update notifications error",
				zap.String("profileId", profileId), zap.Error(err))
		}
		c.Header("X-Speech-Settings-Update", "true")
	}
	middleware.CtxLog(c).Info("speech settings validated",
		zap.String("clientId", clientId), zap.String("profileId", profileId))
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
	reason := "call to ElevenLabs failed"
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

func ParticipantElevenSpeechSettingsHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	update := c.Query("update")
	studyId, upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if studyId == "" || upn == "" {
		// not in a study, so no settings
		c.Status(http.StatusNoContent)
		return
	}
	p, err := storage.GetStudyParticipant(studyId, upn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if p == nil {
		// shouldn't happen
		middleware.CtxLog(c).Info("participant data not found",
			zap.String("profileId", profileId), zap.String("studyId", studyId), zap.String("upn", upn))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database inconsistency"})
		return
	}
	if p.ApiKey == "" {
		// the user is a participant, but they don't have speech settings
		c.Status(http.StatusNoContent)
		return
	}
	settings := services.ElevenLabsGenerateSettings(p.ApiKey, p.VoiceId, p.VoiceName)
	// now get the current profile speech settings and compare them
	profileSettings, err := storage.GetSpeechSettings(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if profileSettings != nil && profileSettings.Settings == settings {
		c.Status(http.StatusNoContent)
		return
	}
	// the participant settings and the user settings are different
	if update == "true" {
		// replace the user settings with the participant settings
		if _, err := storage.UpdateSpeechSettings(profileId, p.ApiKey, p.VoiceId, p.VoiceName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
			return
		}
		if err := storage.ProfileClientSpeechDidUpdate(p.ProfileId, clientId); err != nil {
			middleware.CtxLog(c).Info("ignoring update notifications error",
				zap.String("profileId", p.ProfileId), zap.Error(err))
		}
		c.Header("X-Speech-Settings-Update", "true")
	}
	c.JSON(http.StatusOK, gin.H{"update": update})
}
