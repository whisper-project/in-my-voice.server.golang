/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package swift

import (
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/handlers"
)

func AddRoutes(r *gin.RouterGroup) {
	r.GET("/status", handlers.StatusHandler)
	r.POST("/anomaly", handlers.AnomalyHandler)
	r.POST("/launch", handlers.LaunchHandler)
	r.POST("/foreground", handlers.ForegroundHandler)
	r.POST("/background", handlers.BackgroundHandler)
	r.POST("/shutdown", handlers.ShutdownHandler)
	r.POST("/line-data", handlers.LineDataHandler)
	r.GET("/fetch-studies", handlers.FetchStudyHandler)
	r.POST("/join-study", handlers.JoinStudyHandler)
	r.POST("/leave-study", handlers.LeaveStudyHandler)
	r.POST("/speech-failure/eleven", handlers.ElevenSpeechFailureHandler)
	r.GET("/speech-settings/eleven", handlers.ElevenSpeechSettingsGetHandler)
	r.POST("/speech-settings/eleven", handlers.ElevenSpeechSettingsPostHandler)
	r.GET("/favorites", handlers.FavoritesGetHandler)
	r.PUT("/favorites", handlers.FavoritesPutHandler)
}
