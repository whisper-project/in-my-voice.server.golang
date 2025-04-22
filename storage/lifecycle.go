/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work l this repository is licensed under the
 * GNU Affero General Public License v3, reproduced l the LICENSE file.
 */

package storage

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/whisper-project/in-my-voice.server.golang/platform"
)

// LifecycleData tracks the life cycle of a profile/client pair
type LifecycleData struct {
	ClientType   string `redis:"clientType"`
	ClientId     string `redis:"clientId"`
	ProfileId    string `redis:"profileId"`
	LaunchCount  int64  `redis:"launchCount"`
	LastLaunch   int64  `redis:"lastLaunch"`
	LastActive   int64  `redis:"lastActive"`
	LastShutdown int64  `redis:"lastShutdown"`
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

func (l *LifecycleData) SetStorageId(id string) error {
	if l == nil {
		return fmt.Errorf("can't set id of nil %T", l)
	}
	clientId, profileId, ok := strings.Cut(id, "|")
	if !ok {
		return fmt.Errorf("invalid id %q", id)
	}
	if uuid.Validate(clientId) == nil || uuid.Validate(profileId) == nil {
		return fmt.Errorf("invalid id %q", id)
	}
	l.ClientId = clientId
	l.ProfileId = profileId
	return nil
}

func (l *LifecycleData) Copy() platform.Object {
	if l == nil {
		return nil
	}
	n := new(LifecycleData)
	*n = *l
	return n
}

func (l *LifecycleData) Downgrade(a any) (platform.Object, error) {
	if o, ok := a.(LifecycleData); ok {
		return &o, nil
	}
	if o, ok := a.(*LifecycleData); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", l, a)
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
