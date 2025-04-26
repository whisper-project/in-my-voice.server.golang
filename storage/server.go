/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"context"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ServerId      = uuid.NewString()
	ServerLogger  *zap.Logger
	ServerContext = context.Background() // for when the server isn't running
	ServerPrefix  string
	AdminGuiPath  string
)

func sLog() *zap.Logger {
	return ServerLogger
}

func sCtx() context.Context {
	return ServerContext
}

func BuildServerPrefix() error {
	config := platform.GetConfig()
	scheme, host, port := config.HttpScheme, config.HttpHost, config.HttpPort
	if host == "" {
		return fmt.Errorf("invalid server host: %s", host)
	}
	if port == 0 {
		return fmt.Errorf("invalid server port: %d", port)
	}
	switch scheme {
	case "http":
		if config.HttpPort == 80 {
			ServerPrefix = scheme + "://" + config.HttpHost
		} else {
			ServerPrefix = fmt.Sprintf("%s://%s:%d", scheme, host, port)
		}
	case "https":
		if config.HttpPort == 443 {
			ServerPrefix = scheme + "://" + config.HttpHost
		} else {
			ServerPrefix = fmt.Sprintf("%s://%s:%d", scheme, host, port)
		}
	default:
		return fmt.Errorf("invalid server scheme: %s", scheme)
	}
	return nil
}
