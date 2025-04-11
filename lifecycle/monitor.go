/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package lifecycle

import (
	"context"
	"errors"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"time"
)

func startMonitors() func() error {
	sLog().Info("Starting monitors...")
	stopChannel := make(chan any)
	updateChannel := make(chan any)
	// update channel is closed except when we're in the middle of an update
	close(updateChannel)
	go func() {
		timer := time.NewTicker(1 * time.Hour)
		ctx, cancel := context.WithCancel(sCtx())
		defer cancel()
		doUpdate := func() {
			updateChannel = make(chan any)
			defer close(updateChannel)
			updateMonitors(ctx)
		}
		for {
			go doUpdate()
			select {
			case <-stopChannel:
				cancel()
				timer.Stop()
				return
			case <-timer.C:
				continue
			}
		}
	}()
	return func() error {
		close(stopChannel)
		// give any updating monitors a few seconds to finish
		ctx, cancel := context.WithTimeout(sCtx(), 10*time.Second)
		defer cancel()
		select {
		case <-updateChannel:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func updateMonitors(ctx context.Context) {
	monitors, err := storage.FetchMonitorsForUpdate(ctx)
	if err != nil {
		return
	}
	for _, monitor := range monitors {
		select {
		case <-ctx.Done():
			sLog().Info("Update monitor context has been canceled")
			break
		default:
			if err = monitor.Update(ctx); errors.Is(err, services.ElevenInvalidApiKeyError) {
				_ = storage.RemoveMonitor(monitor.ProfileId)
			}
		}
	}
}
