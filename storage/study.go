/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"errors"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"go.uber.org/zap"
)

type StudyParticipant struct {
	Upn     string `redis:"upn"`
	ApiKey  string `redis:"apiKey"`
	VoiceId string `redis:"voiceId"`
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

func (s *StudyParticipant) SetStorageId(id string) error {
	if s == nil {
		return fmt.Errorf("can't set id of nil %T", s)
	}
	s.Upn = id
	return nil
}

func (s *StudyParticipant) Copy() platform.StructPointer {
	if s == nil {
		return nil
	}
	n := new(StudyParticipant)
	*n = *s
	return n
}

func (s *StudyParticipant) Downgrade(a any) (platform.StructPointer, error) {
	if o, ok := a.(StudyParticipant); ok {
		return &o, nil
	}
	if o, ok := a.(*StudyParticipant); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", s, a)
}

func NewParticipant(upn, apiKey, voiceId string) (*StudyParticipant, error) {
	if ok, err := services.ElevenValidateApiKey(apiKey); err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	}
	if ok, err := services.ElevenValidateVoiceId(apiKey, voiceId); err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	}
	return &StudyParticipant{Upn: upn, ApiKey: apiKey, VoiceId: voiceId}, nil
}

var (
	availableStudyParticipants    = platform.StorableSet("available-study-participants")
	usedStudyParticipants         = platform.StorableSet("used-study-participants")
	unassignedStudyParticipants   = platform.StorableSet("unassigned-study-participants")
	profileParticipantMap         = platform.StorableMap("profile-participant-map")
	ParticipantAlreadyExistsError = errors.New("participant UPN already exists")
	ParticipantNotValidError      = errors.New("apiKey or voiceId not valid")
	ParticipantNotAvailableError  = errors.New("participant UPN not available")
)

func AddStudyParticipant(upn, apiKey, voiceId string) error {
	// make sure this is a new UPN
	if ok, err := platform.IsMember(sCtx(), availableStudyParticipants, upn); err != nil {
		sLog().Error("check member failure on UPN lookup",
			zap.String("studyId", upn), zap.Error(err))
		return err
	} else if ok {
		return ParticipantAlreadyExistsError
	}
	if ok, err := platform.IsMember(sCtx(), usedStudyParticipants, upn); err != nil {
		sLog().Error("check member failure on UPN lookup",
			zap.String("studyId", upn), zap.Error(err))
		return err
	} else if ok {
		return ParticipantAlreadyExistsError
	}
	n, err := NewParticipant(upn, apiKey, voiceId)
	if err != nil {
		return err
	}
	if n == nil {
		return ParticipantNotValidError
	}
	if err := platform.SaveFields(sCtx(), n); err != nil {
		sLog().Error("save fields failure adding participant",
			zap.String("studyId", upn), zap.Error(err))
		return err
	}
	if err := platform.AddMembers(sCtx(), availableStudyParticipants, upn); err != nil {
		sLog().Error("add member failure adding participant",
			zap.String("studyId", upn), zap.Error(err))
		return err
	}
	return nil
}

func GetProfileStudyMembership(profileId string) (string, error) {
	upn, err := platform.MapGet(sCtx(), profileParticipantMap, profileId)
	if err != nil {
		sLog().Error("map get failure on profile lookup",
			zap.String("profileId", profileId), zap.Error(err))
		return "", err
	}
	return upn, nil
}

func AssignStudyParticipant(profileId, upn string) (string, error) {
	if ok, err := platform.IsMember(sCtx(), availableStudyParticipants, upn); err != nil {
		sLog().Error("move one failure on participant assignment",
			zap.String("profileId", profileId), zap.Error(err))
		return "", err
	} else if !ok {
		return "", ParticipantNotAvailableError
	}
	if err := platform.AddMembers(sCtx(), usedStudyParticipants, upn); err != nil {
		sLog().Error("add member failure on participant assignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return "", err
	}
	if err := platform.RemoveMember(sCtx(), availableStudyParticipants, upn); err != nil {
		sLog().Error("remove member failure on participant assignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
	}
	p := &StudyParticipant{Upn: upn}
	if err := platform.LoadFields(sCtx(), p); err != nil {
		sLog().Error("load fields failure on participant assignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return "", err
	}
	if err := platform.MapSet(sCtx(), profileParticipantMap, profileId, upn); err != nil {
		sLog().Error("map set failure on participant assignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
	}
	s := services.ElevenLabsGenerateSettings(p.ApiKey, p.VoiceId)
	return s, nil
}

func UnassignStudyParticipant(profileId string, upn string) error {
	if err := platform.MapRemove(sCtx(), profileParticipantMap, profileId); err != nil {
		sLog().Error("map remove failure on participant unassignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return err
	}
	if err := platform.AddMembers(sCtx(), unassignedStudyParticipants, upn); err != nil {
		sLog().Error("add member failure on participant unassignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return err
	}
	if err := platform.RemoveMember(sCtx(), usedStudyParticipants, upn); err != nil {
		sLog().Error("remove member failure on participant unassignment",
			zap.String("profileId", profileId), zap.String("studyId", upn),
			zap.Error(err))
		return err
	}
	return nil
}
