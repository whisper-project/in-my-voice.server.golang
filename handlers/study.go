/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
	"net/http"
)

func JoinStudyHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	var body map[string]any
	if err := c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	studyId, ok := body["studyId"].(string)
	if !ok || studyId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	if upn, err := storage.GetProfileStudyMembership(profileId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	} else if upn != "" {
		middleware.CtxLog(c).Info("profile already enrolled in study",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", studyId))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "study ID already assigned"})
	}
	settings, apiKey, err := storage.EnrollStudyParticipant(profileId, studyId)
	if errors.Is(err, storage.ParticipantNotAvailableError) {
		middleware.CtxLog(c).Info("study ID invalid or not available",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", studyId))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "study ID invalid or not available"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// user is now in the study, save their settings and tell them
	_, err = storage.UpdateSpeechSettings(profileId, settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	defer func() {
		_ = storage.ProfileClientSpeechDidUpdate(profileId, clientId)
		_ = storage.EnsureMonitor(profileId, apiKey)
	}()
	middleware.CtxLog(c).Info("study ID assigned",
		zap.String("clientId", clientId), zap.String("profileId", profileId),
		zap.String("studyId", studyId))
	c.Header("X-Study-Membership-Update", "true")
	c.Header("X-Speech-Settings-Update", "true")
	c.Status(http.StatusNoContent)
}

func LeaveStudyHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	} else if upn == "" {
		middleware.CtxLog(c).Info("profile isn't enrolled in study",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", upn))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "no study ID assigned"})
	}
	if err = storage.UnenrollStudyParticipant(profileId, upn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// user is now out of the study, but they retain their speech settings
	middleware.CtxLog(c).Info("study ID unassigned",
		zap.String("clientId", clientId), zap.String("profileId", profileId), zap.String("studyId", upn))
	c.Header("X-Study-Membership-Update", "false")
	c.Status(http.StatusNoContent)
}

func LineDataHandler(c *gin.Context) {
	_, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	inStudy := true
	if upn == "" {
		if !storage.GetStudyPolicies().CollectNonStudyStats {
			middleware.CtxLog(c).Info("Refusing line data for non-study profile.",
				zap.String("profileId", profileId))
			c.Header("X-Non-Study-Collect-Stats-Update", "false")
			c.Status(http.StatusNoContent)
			return
		}
		inStudy = false
		if storage.GetStudyPolicies().AnonymizeNonStudyLineStats {
			upn = "NS:anonymous"
		} else {
			upn = "NS:" + profileId
		}
	}
	var platform storage.Platform = storage.PlatformUnknown
	switch header := c.GetHeader("X-Platform-Info"); header {
	case "phone":
		platform = storage.PlatformPhone
	case "pad", "tablet":
		platform = storage.PlatformTablet
	case "mac", "windows", "linux", "android":
		platform = storage.PlatformComputer
	case "web":
		platform = storage.PlatformBrowser
	}
	var body []map[string]any
	if err := c.ShouldBind(&body); err != nil {
		middleware.CtxLog(c).Info("invalid line-data request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	middleware.CtxLog(c).Info("received line-data", zap.Int("count", len(body)))
	go processLineData(inStudy, platform, upn, body)
	c.Status(http.StatusNoContent)
}

func processLineData(inStudy bool, platform storage.Platform, upn string, body []map[string]any) {
	lines := make([]storage.TypedLineStat, len(body))
	var j = 0
	for _, data := range body {
		if isFavorite, ok := data["isFavorite"].(bool); ok {
			if text, ok := data["text"].(string); ok {
				saveRepeatLine(text, isFavorite, inStudy)
			}
		} else {
			if ok := fillStat(&lines[j], platform, upn, data); ok {
				j++
			}
		}
	}
	if j > 0 {
		_ = storage.TypedLineStatList(upn).PushRange(lines[0:j])
	}
}

func fillStat(stat *storage.TypedLineStat, platform storage.Platform, upn string, data map[string]any) bool {
	if completed, ok := data["completed"].(float64); ok {
		if changes, ok := data["changes"].(float64); ok {
			if duration, ok := data["duration"].(float64); ok {
				if length, ok := data["length"].(float64); ok {
					stat.From = platform
					stat.Upn = upn
					stat.Completed = int64(completed)
					stat.Changes = int64(changes)
					stat.Duration = int64(duration)
					stat.Length = int64(length)
					return true
				}
			}
		}
	}
	return false
}

func saveRepeatLine(text string, isFavorite bool, inStudy bool) {
	stat, err := storage.GetOrCreateCannedLineStat(text, inStudy)
	if err != nil {
		return
	}
	if isFavorite {
		stat.FavoriteCount++
	} else {
		stat.RepeatCount++
	}
	_ = storage.SaveCannedLineStat(stat)
}
