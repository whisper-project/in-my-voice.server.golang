/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work l this repository is licensed under the
 * GNU Affero General Public License v3, reproduced l the LICENSE file.
 */

package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/whisper-project/in-my-voice.server.golang/platform"
)

// LifecycleData tracks the life cycle of a profile/client pair
type LifecycleData struct {
	ClientType   string
	ClientId     string
	ProfileId    string
	LaunchCount  int64
	LastLaunch   int64
	LastActive   int64
	LastShutdown int64
}

func (l *LifecycleData) StoragePrefix() string {
	return "launch-data:"
}
func (l *LifecycleData) StorageId() string {
	if l == nil {
		return ""
	}
	return l.ClientId + "|" + l.ProfileId
}
func (l *LifecycleData) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(l); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (l *LifecycleData) FromRedis(b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(l)
}

func NewLifecycleData(clientId, profileId string) *LifecycleData {
	return &LifecycleData{
		ClientId:  clientId,
		ProfileId: profileId,
	}
}

func ObserveClientLaunch(clientType, clientId, profileId string) {
	l := &LifecycleData{ClientId: clientId, ProfileId: profileId}
	if err := platform.LoadObject(sCtx(), l); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			l = NewLifecycleData(clientId, profileId)
		} else {
			sLog().Error("db failure on client launch",
				zap.String("clientType", clientType), zap.String("clientId", clientId),
				zap.String("profileId", profileId), zap.Error(err))
			return
		}
	}
	l.ClientType = clientType
	l.LaunchCount++
	l.LastLaunch = time.Now().UnixMilli()
	l.LastActive = l.LastLaunch
	if err := platform.SaveObject(sCtx(), l); err != nil {
		sLog().Error("db failure on client launch",
			zap.String("clientType", clientType), zap.String("clientId", clientId),
			zap.String("profileId", profileId), zap.Error(err))
	}
}

func ObserveClientActive(clientId, profileId string) {
	l := &LifecycleData{ClientId: clientId, ProfileId: profileId}
	if err := platform.LoadObject(sCtx(), l); err != nil {
		sLog().Error("db failure on client active",
			zap.String("profileId", profileId), zap.Error(err))
		return
	}
	l.LastActive = time.Now().UnixMilli()
	if err := platform.SaveObject(sCtx(), l); err != nil {
		sLog().Error("db failure on client active",
			zap.String("profileId", profileId), zap.Error(err))
	}
}

func ObserveClientShutdown(clientId, profileId string) {
	l := &LifecycleData{ClientId: clientId, ProfileId: profileId}
	if err := platform.LoadObject(sCtx(), l); err != nil {
		sLog().Error("db failure on client shutdown",
			zap.String("profileId", profileId), zap.Error(err))
		return
	}
	l.LastShutdown = time.Now().UnixMilli()
	if err := platform.SaveObject(sCtx(), l); err != nil {
		sLog().Error("db failure on client shutdown",
			zap.String("profileId", profileId), zap.Error(err))
	}
}

type NotifiedSpeechClients string

func (n NotifiedSpeechClients) StoragePrefix() string {
	return "notified-speech-clients:"
}

func (n NotifiedSpeechClients) StorageId() string {
	return string(n)
}

func ProfileClientSpeechDidUpdate(profileId, clientId string) error {
	n := NotifiedSpeechClients(profileId)
	if err := platform.DeleteStorage(sCtx(), n); err != nil {
		sLog().Error("delete storage failure", zap.Error(err))
		return err
	}
	if err := platform.AddMembers(sCtx(), n, clientId); err != nil {
		sLog().Error("add set member failed", zap.Error(err))
		return err
	}
	return nil
}

func ProfileClientSpeechNeedsNotification(profileId, clientId string) (bool, error) {
	n := NotifiedSpeechClients(profileId)
	isMember, err := platform.IsMember(sCtx(), n, clientId)
	if err != nil {
		sLog().Error("lookup set member failed", zap.Error(err))
		return false, err
	}
	return !isMember, nil
}

func ProfileClientSpeechWasNotified(profileId, clientId string) error {
	n := NotifiedSpeechClients(profileId)
	if err := platform.AddMembers(sCtx(), n, clientId); err != nil {
		sLog().Error("add set member failed", zap.Error(err))
		return err
	}
	return nil
}

type NotifiedFavoritesClients string

func (n NotifiedFavoritesClients) StoragePrefix() string {
	return "notified-speech-clients:"
}

func (n NotifiedFavoritesClients) StorageId() string {
	return string(n)
}

func ProfileClientFavoritesDidUpdate(profileId, clientId string) error {
	n := NotifiedFavoritesClients(profileId)
	if err := platform.DeleteStorage(sCtx(), n); err != nil {
		sLog().Error("delete storage failure", zap.Error(err))
		return err
	}
	if err := platform.AddMembers(sCtx(), n, clientId); err != nil {
		sLog().Error("add set member failed", zap.Error(err))
		return err
	}
	return nil
}

func ProfileClientFavoritesNeedsNotification(profileId, clientId string) (bool, error) {
	n := NotifiedFavoritesClients(profileId)
	isMember, err := platform.IsMember(sCtx(), n, clientId)
	if err != nil {
		sLog().Error("lookup set member failed", zap.Error(err))
		return false, err
	}
	return !isMember, nil
}

func ProfileClientFavoritesWasNotified(profileId, clientId string) error {
	n := NotifiedFavoritesClients(profileId)
	if err := platform.AddMembers(sCtx(), n, clientId); err != nil {
		sLog().Error("add set member failed", zap.Error(err))
		return err
	}
	return nil
}
