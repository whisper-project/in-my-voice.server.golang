/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
)

func FetchStudyHandler(c *gin.Context) {
	_, _, ok := ValidateRequest(c)
	if !ok {
		return
	}
	studies, err := storage.GetAllStudies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
	}
	results := make(map[string]string, len(studies))
	for _, study := range studies {
		results[study.Name] = study.Id
	}
	c.JSON(http.StatusOK, results)
}

func JoinStudyHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	studyId, upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if studyId != "" || upn != "" {
		middleware.CtxLog(c).Info("profile already enrolled in study",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", studyId), zap.String("upn", upn))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "study ID already assigned"})
	}
	var body map[string]any
	if err = c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	studyId, _ = body["studyId"].(string)
	upn, _ = body["upn"].(string)
	if studyId == "" || upn == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	study, err := storage.GetStudy(studyId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if study == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "study ID invalid or not available"})
		return
	}
	p, err := storage.EnrollStudyParticipant(profileId, studyId, upn)
	if errors.Is(err, storage.ParticipantNotAvailableError) {
		middleware.CtxLog(c).Info("UPN invalid or not available",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", studyId))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "UPN invalid or not available"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// user is now in the study
	middleware.CtxLog(c).Info("user assigned to study",
		zap.String("clientId", clientId), zap.String("profileId", profileId),
		zap.String("studyId", studyId), zap.String("upn", upn))
	c.Header("X-Study-Membership-Update", study.Name)
	// if there are speech settings in the participant, apply them to the profile
	if p != nil && p.ApiKey != "" {
		updatedUser, err := storage.UpdateSpeechSettings(profileId, p.ApiKey, p.VoiceId, p.VoiceName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
			return
		}
		if updatedUser {
			// ignore update errors
			_ = storage.ProfileClientSpeechDidUpdate(profileId, clientId)
			c.Header("X-Speech-Settings-Update", "true")
		}
		c.JSON(http.StatusOK, gin.H{"elevenSettings": "updated"})
		return
	}
	c.Status(http.StatusNoContent)
}

func LeaveStudyHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	studyId, upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	} else if upn == "" {
		middleware.CtxLog(c).Info("profile isn't enrolled in study",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", upn))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "no study ID assigned"})
	}
	if err = storage.UnenrollStudyParticipant(profileId, studyId, upn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// user is now out of the study, but they retain their speech settings
	middleware.CtxLog(c).Info("study ID unassigned",
		zap.String("clientId", clientId), zap.String("profileId", profileId), zap.String("studyId", upn))
	c.Header("X-Study-Membership-Update", "none")
	c.Status(http.StatusNoContent)
}

func LineDataHandler(c *gin.Context) {
	_, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	studyId, upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
		return
	}
	if studyId == "" || upn == "" {
		// no data kept for non-study participants
		return
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
	go processLineData(platform, studyId, upn, body)
	c.Status(http.StatusNoContent)
}

func processLineData(platform storage.Platform, studyId, upn string, body []map[string]any) {
	lines := make([]storage.TypedLineStat, len(body))
	var j = 0
	for _, data := range body {
		if isFavorite, ok := data["isFavorite"].(bool); ok {
			if text, ok := data["text"].(string); ok {
				saveRepeatLine(studyId, text, isFavorite)
			}
		} else {
			if ok := fillStat(&lines[j], platform, upn, data); ok {
				j++
			}
		}
	}
	if j > 0 {
		_ = storage.StudyTypedLineStatsIndex(studyId + "+" + upn).PushRange(lines[0:j])
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

func saveRepeatLine(studyId, text string, isFavorite bool) {
	stat, err := storage.GetOrCreatePhraseStat(studyId, text)
	if err != nil {
		return
	}
	if isFavorite {
		stat.FavoriteCount++
	} else {
		stat.RepeatCount++
	}
	_ = storage.SavePhraseStat(studyId, stat)
}
