/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"encoding/gob"
	"errors"

	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
)

type SpeechSettings struct {
	ProfileId string
	ApiKey    string
	VoiceId   string
	VoiceName string
	ModelId   string
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
func (s *SpeechSettings) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *SpeechSettings) FromRedis(b []byte) error {
	*s = SpeechSettings{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

func GetSpeechSettings(profileId string) (*SpeechSettings, error) {
	s := &SpeechSettings{ProfileId: profileId}
	if err := platform.LoadObject(sCtx(), s); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			return nil, nil
		}
		sLog().Error("db failure on settings fetch",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	return s, nil
}

func MatchesCurrentSpeechSettings(profileId, apiKey, voiceId string) (bool, error) {
	o, err := GetSpeechSettings(profileId)
	if err != nil {
		sLog().Error("db failure on speech settings compare",
			zap.String("profileId", profileId), zap.Error(err))
		return false, err
	}
	if o == nil {
		return false, nil
	}
	return apiKey == o.ApiKey && voiceId == o.VoiceId, nil
}

func UpdateSpeechSettings(profileId, apiKey, voiceId, voiceName, modelId string) (bool, error) {
	o, err := GetSpeechSettings(profileId)
	if err != nil {
		sLog().Error("db failure on speech settings update",
			zap.String("profileId", profileId), zap.Error(err))
		return false, err
	}
	if o != nil {
		if modelId == "" {
			modelId = o.ModelId
		}
		// ignore voice name when comparing, because it's determined by voiceId
		if apiKey == o.ApiKey && voiceId == o.VoiceId && modelId == o.ModelId {
			return false, nil
		}
		o.ApiKey, o.VoiceId, o.VoiceName, o.ModelId = apiKey, voiceId, voiceName, modelId
	} else {
		o = &SpeechSettings{ProfileId: profileId, ApiKey: apiKey, VoiceId: voiceId, VoiceName: voiceName, ModelId: modelId}
	}
	if err := platform.SaveObject(sCtx(), o); err != nil {
		sLog().Error("db failure on settings update",
			zap.String("profileId", profileId), zap.Error(err))
		return false, err
	}
	if err := EnsureMonitor(profileId, apiKey); err != nil {
		sLog().Info("ignoring monitor update failure",
			zap.String("profileId", profileId), zap.Error(err))
	}
	return true, nil
}

func DeleteSpeechSettings(profileId string) error {
	s := &SpeechSettings{ProfileId: profileId}
	if err := platform.DeleteStorage(sCtx(), s); err != nil {
		sLog().Error("delete failure on settings delete",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	if err := RemoveMonitor(profileId); err != nil {
		sLog().Info("ignoring monitor removal error",
			zap.String("profileId", profileId), zap.Error(err))
	}
	return nil
}
