/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ServerId      = uuid.NewString()
	ServerLogger  *zap.Logger
	ServerContext context.Context = context.Background() // for when the server isn't running
)

func sLog() *zap.Logger {
	return ServerLogger
}

func sCtx() context.Context {
	return ServerContext
}
