/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func ChangeDataHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func RepeatLineHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
