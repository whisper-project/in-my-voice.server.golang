/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package middleware

import (
	"github.com/gin-gonic/gin"
	"regexp"
)

var rootPath = regexp.MustCompile("^/[^/]+$")

func RewriteRoot(r *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rootPath.MatchString(c.Request.URL.Path) {
			c.Request.URL.Path = "/root" + c.Request.URL.Path
			r.HandleContext(c)
			return
		}
		c.AbortWithStatus(404)
	}
}
