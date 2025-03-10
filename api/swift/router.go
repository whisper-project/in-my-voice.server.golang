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
	r.POST("/launch", handlers.LaunchHandler)
	r.GET("/shutdown", handlers.ShutdownHandler)
}
