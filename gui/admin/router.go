/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/handlers"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
)

func AddRoutes(r *gin.RouterGroup) {
	storage.AdminGuiPath = r.BasePath()
	r.GET("/login", handlers.GetLoginHandler)
	r.POST("/login", handlers.PostLoginHandler)
	r.GET("/logout", handlers.LogoutHandler)
	r.GET("/:sessionId/logout", handlers.LogoutHandler)
	r.GET("/:sessionId/home", handlers.AuthMiddleware, handlers.GetHomeHandler)
	r.POST("/:sessionId/home", handlers.AuthMiddleware, handlers.PostHomeHandler)
	r.GET("/:sessionId/users", handlers.AuthMiddleware, handlers.GetUsersHandler)
	r.POST("/:sessionId/users", handlers.AuthMiddleware, handlers.PostUsersHandler)
	r.GET("/:sessionId/participants", handlers.AuthMiddleware, handlers.GetParticipantsHandler)
	r.POST("/:sessionId/participants", handlers.AuthMiddleware, handlers.PostParticipantsHandler)
	r.GET("/:sessionId/reports", handlers.AuthMiddleware, handlers.GetReportsHandler)
	r.POST("/:sessionId/reports", handlers.AuthMiddleware, handlers.PostReportsHandler)
	r.GET("/:sessionId/admins", handlers.AuthMiddleware, handlers.GetAdminsHandler)
	r.POST("/:sessionId/admins", handlers.AuthMiddleware, handlers.PostAdminsHandler)
	r.GET("/:sessionId/studies", handlers.AuthMiddleware, handlers.GetStudiesHandler)
	r.POST("/:sessionId/studies", handlers.AuthMiddleware, handlers.PostStudiesHandler)
	r.GET("/:sessionId/download-report/:reportId", handlers.AuthMiddleware, handlers.DownloadReportHandler)
}
