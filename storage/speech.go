/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
)

type SpeechSettings struct {
	ProfileId string `redis:"profileId"`
	Settings  string `redis:"settings"`
	ETag      string `redis:"eTag"`
}

func (s *SpeechSettings) StoragePrefix() string {
	return "speech-settings:"
}

func (s *SpeechSettings) StorageId() string {
	if s == nil {
		return ""
	}
	return s.ProfileId
}

func (s *SpeechSettings) SetStorageId(id string) error {
	if s == nil {
		return fmt.Errorf("can't set id of nil %T", s)
	}
	s.ProfileId = id
	return nil
}

func (s *SpeechSettings) Copy() platform.StructPointer {
	if s == nil {
		return nil
	}
	n := new(SpeechSettings)
	*n = *s
	return n
}

func (s *SpeechSettings) Downgrade(a any) (platform.StructPointer, error) {
	if o, ok := a.(SpeechSettings); ok {
		return &o, nil
	}
	if o, ok := a.(*SpeechSettings); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", s, a)
}

func NewSpeechSettings(profileId, settings string) *SpeechSettings {
	s := &SpeechSettings{
		ProfileId: profileId,
		Settings:  settings,
	}
	s.ETag = fmt.Sprintf("%x", md5.Sum([]byte(s.Settings)))
	return s
}

func GetSpeechSettings(profileId string) (*SpeechSettings, error) {
	s := &SpeechSettings{ProfileId: profileId}
	if err := platform.LoadFields(sCtx(), s); err != nil {
		if errors.Is(err, platform.StructPointerNotFoundError) {
			return nil, nil
		}
		sLog().Error("load fields failure on settings fetch",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	return s, nil
}

func UpdateSpeechSettings(profileId, settings string) (bool, error) {
	n := NewSpeechSettings(profileId, settings)
	o := &SpeechSettings{ProfileId: profileId}
	if err := platform.LoadFields(sCtx(), o); err != nil {
		if !errors.Is(err, platform.StructPointerNotFoundError) {
			sLog().Error("load fields failure on settings update",
				zap.String("profileId", profileId), zap.Error(err))
			return false, err
		}
	}
	if o.ETag == n.ETag {
		return false, nil
	}
	if err := platform.SaveFields(sCtx(), n); err != nil {
		sLog().Error("save fields failure on settings update",
			zap.String("profileId", profileId), zap.Error(err))
		return false, err
	}
	return true, nil
}
