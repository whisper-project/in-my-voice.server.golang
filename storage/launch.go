/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work l this repository is licensed under the
 * GNU Affero General Public License v3, reproduced l the LICENSE file.
 */

package storage

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/whisper-project/in-my-voice.server.golang/platform"
)

// LaunchData tracks the last launch of a client
type LaunchData struct {
	ClientType string `redis:"clientType"`
	ClientId   string `redis:"clientId"`
	ProfileId  string `redis:"profileId"`
	Start      int64  `redis:"start"`
	End        int64  `redis:"end"`
}

func (l *LaunchData) StoragePrefix() string {
	return "launch-data:"
}

func (l *LaunchData) StorageId() string {
	if l == nil {
		return ""
	}
	return l.ClientId
}

func (l *LaunchData) SetStorageId(id string) error {
	if l == nil {
		return fmt.Errorf("can't set id of nil %T", l)
	}
	l.ClientId = id
	return nil
}

func (l *LaunchData) Copy() platform.StructPointer {
	if l == nil {
		return nil
	}
	n := new(LaunchData)
	*n = *l
	return n
}

func (l *LaunchData) Downgrade(a any) (platform.StructPointer, error) {
	if o, ok := a.(LaunchData); ok {
		return &o, nil
	}
	if o, ok := a.(*LaunchData); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", l, a)
}

func NewLaunchData(clientType, clientId, profileId string) *LaunchData {
	return &LaunchData{
		ClientType: clientType,
		ClientId:   clientId,
		ProfileId:  profileId,
		Start:      time.Now().UnixMilli(),
	}
}

type ClientWhisperConversations string

func (c ClientWhisperConversations) StoragePrefix() string {
	return "client-whisper-conversations:"
}

func (c ClientWhisperConversations) StorageId() string {
	return string(c)
}

func ObserveClientLaunch(clientType, clientId, profileId string) {
	l := NewLaunchData(clientType, clientId, profileId)
	if err := platform.SaveFields(sCtx(), l); err != nil {
		sLog().Error("save fields failure on client launch",
			zap.String("clientType", clientType), zap.String("clientId", clientId),
			zap.String("profileId", profileId), zap.Error(err))
	}
}

func ObserveClientShutdown(clientId string) {
	l := &LaunchData{ClientId: clientId}
	if err := platform.LoadFields(sCtx(), l); err != nil {
		sLog().Error("load fields failure on client shutdown",
			zap.String("clientId", clientId), zap.Error(err))
		return
	}
	l.End = time.Now().UnixMilli()
	if err := platform.SaveFields(sCtx(), l); err != nil {
		sLog().Error("save fields failure on client shutdown",
			zap.String("clientId", clientId), zap.Error(err))
	}
}
