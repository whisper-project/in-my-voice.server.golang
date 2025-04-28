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
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"go.uber.org/zap"
	"time"
)

type StudyParticipant struct {
	Upn       string
	Memo      string
	Assigned  int64
	ProfileId string
	Started   int64
	Finished  int64
	ApiKey    string
	VoiceId   string
	VoiceName string
}

func (s *StudyParticipant) StoragePrefix() string {
	return "study-participant:"
}
func (s *StudyParticipant) StorageId() string {
	if s == nil {
		return ""
	}
	return s.Upn
}
func (s *StudyParticipant) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *StudyParticipant) FromRedis(b []byte) error {
	*s = StudyParticipant{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

func (s *StudyParticipant) UpdateApiKey(apiKey string) (bool, error) {
	if ok, err := services.ElevenValidateApiKey(apiKey); err != nil {
		return false, err
	} else if !ok {
		return false, nil
	}
	s.ApiKey = apiKey
	if err := platform.SaveObject(sCtx(), s); err != nil {
		sLog().Error("db failure on participant save",
			zap.String("studyId", s.Upn), zap.Error(err))
		return false, err
	}
	return true, nil
}

func (s *StudyParticipant) UpdateVoiceId(voiceId string) (bool, error) {
	if s.ApiKey == "" {
		return false, nil
	}
	name, ok, err := services.ElevenValidateVoiceId(s.ApiKey, voiceId)
	if err != nil {
		return false, err
	} else if !ok {
		return false, nil
	}
	s.VoiceId = voiceId
	s.VoiceName = name
	if err := platform.SaveObject(sCtx(), s); err != nil {
		sLog().Error("db failure on participant save",
			zap.String("studyId", s.Upn), zap.Error(err))
		return false, err
	}
	return true, nil
}

func (s *StudyParticipant) UpdateAssignment(memo string) error {
	s.Memo = memo
	if s.Assigned == 0 {
		s.Assigned = time.Now().UnixMilli()
	}
	if err := platform.SaveObject(sCtx(), s); err != nil {
		sLog().Error("db failure on participant save",
			zap.String("studyId", s.Upn), zap.Error(err))
		return err
	}
	return nil
}

func GetStudyParticipant(upn string) (*StudyParticipant, error) {
	s := &StudyParticipant{Upn: upn}
	if err := platform.LoadObject(sCtx(), s); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			return nil, nil
		}
		sLog().Error("db failure on participant fetch",
			zap.String("studyId", upn), zap.Error(err))
		return nil, err
	}
	return s, nil
}

func GetAllStudyParticipants() ([]*StudyParticipant, error) {
	s := &StudyParticipant{}
	var result []*StudyParticipant
	collect := func() {
		n := *s
		s = nil
		result = append(result, &n)
	}
	if err := platform.MapObjects(sCtx(), collect, s); err != nil {
		sLog().Error("db failure on participant map", zap.Error(err))
		return nil, err
	}
	return result, nil
}

func CreateStudyParticipant(upn string) (*StudyParticipant, error) {
	existing, err := GetStudyParticipant(upn)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ParticipantAlreadyExistsError
	}
	p := &StudyParticipant{Upn: upn}
	if err = platform.SaveObject(sCtx(), p); err != nil {
		sLog().Error("db failure on participant creation",
			zap.String("studyId", upn), zap.Error(err))
		return nil, err
	}
	return p, nil
}

var (
	profileParticipantMap         = platform.StorableMap("profile-participant-map")
	ParticipantAlreadyExistsError = errors.New("participant UPN already exists")
	ParticipantNotValidError      = errors.New("apiKey or voiceId not valid")
	ParticipantNotAvailableError  = errors.New("participant UPN not available")
)

func GetProfileStudyMembership(profileId string) (string, error) {
	upn, err := platform.MapGet(sCtx(), profileParticipantMap, profileId)
	if err != nil {
		sLog().Error("map get failure on profile lookup",
			zap.String("profileId", profileId), zap.Error(err))
		return "", err
	}
	return upn, nil
}

func EnrollStudyParticipant(profileId, upn string) (settings, apiKey string, err error) {
	var p *StudyParticipant
	p, err = GetStudyParticipant(upn)
	if err != nil {
		return
	}
	if p == nil {
		err = ParticipantNotAvailableError
		return
	}
	if p.Assigned == 0 {
		sLog().Info("can't enroll participant without assignment",
			zap.String("studyId", upn), zap.String("profileId", profileId))
		err = ParticipantNotAvailableError
		return
	}
	if p.ProfileId == "" {
		p.ProfileId = profileId
		p.Started = time.Now().UnixMilli()
	} else if p.ProfileId == profileId {
		// participant is re-enrolling
		p.Finished = 0
	} else {
		err = ParticipantNotAvailableError
		return
	}
	if err = platform.SaveObject(sCtx(), p); err != nil {
		sLog().Error("db failure on participant save",
			zap.String("studyId", upn), zap.Error(err))
		return
	}
	if err = platform.MapSet(sCtx(), profileParticipantMap, profileId, upn); err != nil {
		sLog().Error("map set failure on participant assignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return
	}
	apiKey = p.ApiKey
	settings = services.ElevenLabsGenerateSettings(apiKey, p.VoiceId, p.VoiceName)
	return
}

func UnenrollStudyParticipant(profileId string, upn string) error {
	p, err := GetStudyParticipant(upn)
	if err != nil {
		return err
	}
	if p == nil {
		return ParticipantNotValidError
	}
	if p.ProfileId != profileId {
		return ParticipantNotValidError
	}
	p.Finished = time.Now().UnixMilli()
	if err = platform.SaveObject(sCtx(), p); err != nil {
		sLog().Error("db failure on participant save",
			zap.String("studyId", upn), zap.Error(err))
		return err
	}
	if err := platform.MapRemove(sCtx(), profileParticipantMap, profileId); err != nil {
		sLog().Error("map remove failure on participant unassignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return err
	}
	return nil
}
