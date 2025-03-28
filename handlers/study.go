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
)

func JoinStudyHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	var body map[string]string
	if err := c.ShouldBind(body); err != nil {
		c.JSON(400, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	studyId := body["studyId"]
	if studyId == "" {
		c.JSON(400, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	if upn, err := storage.GetProfileStudyMembership(profileId); err != nil {
		c.JSON(500, gin.H{"status": "error", "error": "database failure"})
		return
	} else if upn != "" {
		middleware.CtxLog(c).Info("profile already enrolled in study",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", studyId))
		c.JSON(403, gin.H{"status": "error", "error": "study ID already assigned"})
	}
	settings, err := storage.AssignStudyParticipant(profileId, studyId)
	if errors.Is(err, storage.ParticipantNotAvailableError) {
		middleware.CtxLog(c).Info("study ID invalid or not available",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", studyId))
		c.JSON(403, gin.H{"status": "error", "error": "study ID invalid or not available"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// user is now in the study, save their settings and tell them
	_, err = storage.UpdateSpeechSettings(profileId, settings)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "error": "database failure"})
		return
	}
	defer func() {
		_ = storage.ProfileClientSpeechDidUpdate(profileId, clientId)
	}()
	middleware.CtxLog(c).Info("study ID assigned",
		zap.String("clientId", clientId), zap.String("profileId", profileId),
		zap.String("studyId", studyId))
	c.Header("X-Study-Membership-Update", "true")
	c.Header("X-Speech-Settings-Update", "true")
	c.Status(204)
}

func LeaveStudyHandler(c *gin.Context) {
	clientId, profileId, ok := ValidateRequest(c)
	if !ok {
		return
	}
	upn, err := storage.GetProfileStudyMembership(profileId)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "error": "database failure"})
		return
	} else if upn == "" {
		middleware.CtxLog(c).Info("profile not enrolled in study",
			zap.String("clientId", clientId), zap.String("profileId", profileId),
			zap.String("studyId", upn))
		c.JSON(403, gin.H{"status": "error", "error": "no study ID assigned"})
	}
	if err = storage.UnassignStudyParticipant(profileId, upn); err != nil {
		c.JSON(500, gin.H{"status": "error", "error": "database failure"})
		return
	}
	// user is now out of the study, delete their settings and tell them
	if err := storage.DeleteSpeechSettings(profileId); err != nil {
		c.JSON(500, gin.H{"status": "error", "error": "database failure"})
	}
	defer func() {
		_ = storage.ProfileClientSpeechDidUpdate(profileId, clientId)
	}()
	middleware.CtxLog(c).Info("study ID unassigned",
		zap.String("clientId", clientId), zap.String("profileId", profileId), zap.String("studyId", upn))
	c.Header("X-Study-Membership-Update", "false")
	c.Header("X-Speech-Settings-Update", "true")
}
