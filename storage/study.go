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
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"go.uber.org/zap"
)

// A Study is the locus of data collection and participant management
//
// Each Study is a value kept in the studyIndex map from studyIds to studies.
type Study struct {
	Id         string
	Name       string
	AdminEmail string
	Active     bool
}

func (s *Study) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *Study) FromRedis(b []byte) error {
	*s = Study{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

var (
	// the global map from a study's id to its study object
	studyIndex = platform.StorableMap("study-index")
)

func (s *Study) Save() error {
	b, err := s.ToRedis()
	if err != nil {
		sLog().Error("serialization failure on study", zap.String("studyId", s.Id), zap.Error(err))
		return err
	}
	if err := platform.MapSet(sCtx(), studyIndex, s.Id, string(b)); err != nil {
		sLog().Error("map set failure on study save", zap.String("studyId", s.Id), zap.Error(err))
		return err
	}
	return nil
}

func GetStudy(id string) (*Study, error) {
	val, err := platform.MapGet(sCtx(), studyIndex, id)
	if err != nil {
		sLog().Error("map get failure on study lookup", zap.String("studyId", id), zap.Error(err))
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var s Study
	if err := s.FromRedis([]byte(val)); err != nil {
		sLog().Error("db failure on study fetch", zap.String("studyId", id), zap.Error(err))
		return nil, err
	}
	return &s, nil
}

func GetAllStudyIds() ([]string, error) {
	keys, err := platform.MapGetKeys(sCtx(), studyIndex)
	if err != nil {
		sLog().Error("map get failure on fetch of all study ids", zap.Error(err))
	}
	return keys, nil
}

func GetAllStudies() ([]*Study, error) {
	result := make([]*Study, 0, len(studyIndex))
	m, err := platform.MapGetAll(sCtx(), studyIndex)
	if err != nil {
		sLog().Error("map get failure on fetch of all studies", zap.Error(err))
		return nil, err
	}
	for _, val := range m {
		var s Study
		if err := s.FromRedis([]byte(val)); err != nil {
			sLog().Error("deserialization failure on study ", zap.String("studyId", s.Id), zap.Error(err))
		}
		result = append(result, &s)
	}
	return result, nil
}

// DeleteStudy will delete everything, including all stats! Be careful!
func DeleteStudy(studyId string) error {
	// first make sure there are no active participants
	participants, err := GetAllStudyParticipants(studyId)
	if err != nil {
		return err
	}
	for _, p := range participants {
		if p.Started > 0 && p.Finished == 0 {
			return ParticipantInUseError
		}
	}
	// next delete all the phrase stats for the participants
	if err = platform.DeleteStorage(sCtx(), PhraseStatsIndex(studyId)); err != nil {
		sLog().Error("db failure on phrase stats delete",
			zap.String("studyId", studyId), zap.Error(err))
		return err
	}
	// next delete all the line stats for the participants
	for _, p := range participants {
		if err = platform.DeleteStorage(sCtx(), StudyTypedLineStatsIndex(studyId+"+"+p.Upn)); err != nil {
			sLog().Error("db failure on typed line stats delete",
				zap.String("studyId", studyId), zap.String("upn", p.Upn), zap.Error(err))
			return err
		}
	}
	// finally, delete all the participants
	if err = platform.DeleteStorage(sCtx(), ParticipantIndex(studyId)); err != nil {
		sLog().Error("db failure on delete of participants",
			zap.String("studyId", studyId), zap.Error(err))
	}
	return nil
}

// The ParticipantIndex of a studyId maps from (lowercase of UPN) to StudyParticipant.
type ParticipantIndex string

func (i ParticipantIndex) StoragePrefix() string {
	return "study-members:"
}
func (i ParticipantIndex) StorageId() string {
	return string(i)
}

type StudyParticipant struct {
	Upn       string
	StudyId   string
	Memo      string
	Assigned  int64
	ProfileId string
	Started   int64
	Finished  int64
	ApiKey    string
	VoiceId   string
	VoiceName string
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

func (s *StudyParticipant) save() error {
	b, err := s.ToRedis()
	if err != nil {
		sLog().Error("serialization failure on participant", zap.String("studyId", s.StudyId), zap.Error(err))
		return err
	}
	// lowercase the UPN to prevent lookup errors
	if err := platform.MapSet(sCtx(), ParticipantIndex(s.StudyId), strings.ToLower(s.Upn), string(b)); err != nil {
		sLog().Error("map set failure on participant save", zap.String("studyId", s.StudyId), zap.Error(err))
	}
	return nil
}

func (s *StudyParticipant) UpdateApiKey(apiKey string) (bool, error) {
	if apiKey == "" {
		s.ApiKey = ""
		s.VoiceId = ""
	} else {
		if ok, err := services.ElevenValidateApiKey(apiKey); err != nil {
			return false, err
		} else if !ok {
			return false, nil
		}
		s.ApiKey = apiKey
	}
	if err := s.save(); err != nil {
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
	if err := s.save(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *StudyParticipant) UpdateAssignment(memo string) error {
	s.Memo = memo
	if s.Assigned == 0 {
		s.Assigned = time.Now().UnixMilli()
	}
	if err := s.save(); err != nil {
		return err
	}
	return nil
}

func GetStudyParticipant(studyId, upn string) (*StudyParticipant, error) {
	// lowercase the UPN to prevent lookup errors
	val, err := platform.MapGet(sCtx(), ParticipantIndex(studyId), strings.ToLower(upn))
	if err != nil {
		sLog().Error("map get failure on participant lookup",
			zap.String("studyId", studyId), zap.String("upn", upn), zap.Error(err))
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var s StudyParticipant
	if err := s.FromRedis([]byte(val)); err != nil {
		sLog().Error("db failure on participant fetch",
			zap.String("studyId", studyId), zap.String("upn", upn), zap.Error(err))
		return nil, err
	}
	return &s, nil
}

// DeleteStudyParticipant should be used with caution because it will also delete
// any collected line stats for this participant.
func DeleteStudyParticipant(studyId, upn string) error {
	s, err := GetStudyParticipant(studyId, upn)
	if err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	if s.Started > 0 && s.Finished == 0 {
		return ParticipantInUseError
	}
	// lowercase the UPN to prevent lookup errors
	if err = platform.MapRemove(sCtx(), ParticipantIndex(studyId), strings.ToLower(upn)); err != nil {
		sLog().Error("db failure on participant delete",
			zap.String("studyId", studyId), zap.String("upn", upn), zap.Error(err))
		return err
	}
	if err = platform.DeleteStorage(sCtx(), StudyTypedLineStatsIndex(studyId+"+"+upn)); err != nil {
		sLog().Error("db failure on typed line stats delete",
			zap.String("studyId", studyId), zap.String("upn", upn), zap.Error(err))
	}
	return nil
}

func GetAllStudyParticipants(studyId string) ([]*StudyParticipant, error) {
	m, err := platform.MapGetAll(sCtx(), ParticipantIndex(studyId))
	if err != nil {
		sLog().Error("db failure fetching participants",
			zap.String("studyId", studyId), zap.Error(err))
		return nil, err
	}
	results := make([]*StudyParticipant, 0, len(m))
	for _, val := range m {
		var s StudyParticipant
		if err = s.FromRedis([]byte(val)); err != nil {
			sLog().Error("deserialization failure on participant ",
				zap.String("studyId", studyId), zap.String("upn", s.Upn), zap.Error(err))
			return nil, err
		}
		results = append(results, &s)
	}
	return results, nil
}

func CreateStudyParticipant(studyId, upn string) (*StudyParticipant, error) {
	existing, err := GetStudyParticipant(studyId, upn)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ParticipantAlreadyExistsError
	}
	p := &StudyParticipant{Upn: upn, StudyId: studyId}
	if err = p.save(); err != nil {
		return nil, err
	}
	return p, nil
}

var (
	profileParticipantMap         = platform.StorableMap("profile-participant-map")
	ParticipantAlreadyExistsError = errors.New("participant UPN already exists")
	ParticipantNotValidError      = errors.New("apiKey or voiceId not valid")
	ParticipantNotAvailableError  = errors.New("participant UPN not available")
	ParticipantInUseError         = errors.New("participant UPN is in use in the app")
)

func GetProfileStudyMembership(profileId string) (studyId string, upn string, err error) {
	var id string
	id, err = platform.MapGet(sCtx(), profileParticipantMap, profileId)
	if err != nil {
		sLog().Error("map get failure on profile lookup",
			zap.String("profileId", profileId), zap.Error(err))
		return
	}
	studyId, upn, _ = strings.Cut(id, "+")
	return
}

func EnrollStudyParticipant(profileId, studyId, upn string) (*StudyParticipant, error) {
	var p *StudyParticipant
	p, err := GetStudyParticipant(studyId, upn)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ParticipantNotAvailableError
	}
	if p.Assigned == 0 {
		sLog().Info("auto-assigning participant",
			zap.String("studyId", studyId), zap.String("upn", upn), zap.String("profileId", profileId))
		p.Assigned = time.Now().UnixMilli()
		p.Memo = "in-app"
	}
	if p.ProfileId == "" {
		p.ProfileId = profileId
		p.Started = time.Now().UnixMilli()
	} else if p.ProfileId == profileId {
		// participant is re-enrolling
		p.Finished = 0
	} else {
		return nil, ParticipantNotAvailableError
	}
	if err = p.save(); err != nil {
		return nil, err
	}
	if err = platform.MapSet(sCtx(), profileParticipantMap, profileId, studyId+"+"+upn); err != nil {
		sLog().Error("map set failure on participant assignment",
			zap.String("profileId", profileId), zap.String("studyId", studyId), zap.String("upn", upn),
			zap.Error(err))
		return nil, err
	}
	return p, nil
}

func UnenrollStudyParticipant(profileId string, studyId, upn string) error {
	p, err := GetStudyParticipant(studyId, upn)
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
	if err = p.save(); err != nil {
		return err
	}
	if err = platform.MapRemove(sCtx(), profileParticipantMap, profileId); err != nil {
		sLog().Error("map remove failure on participant unassignment",
			zap.String("profileId", profileId), zap.String("studyId", studyId), zap.String("upn", upn),
			zap.Error(err))
		return err
	}
	return nil
}

type Platform = uint8

const (
	PlatformUnknown = iota
	PlatformPhone
	PlatformTablet
	PlatformComputer
	PlatformBrowser
)

var PlatformNames = []string{"Unknown", "Phone", "Tablet", "Computer", "Browser"}

// A StudyTypedLineStatsIndex is a study's map from UPN to the list of TypedLineStat values for that participant.
//
// Rather than keeping this as a hash table keyed by UPN, where each value is a list, we instead keep each list
// as a Redis list and use <studyId>+<UPN> as the Redis key for that value. This allows us to use the Redis
// list commands to efficiently push new stats
type StudyTypedLineStatsIndex string

func (i StudyTypedLineStatsIndex) StoragePrefix() string {
	return "typed-line-stat-list:"
}
func (i StudyTypedLineStatsIndex) StorageId() string {
	return string(i)
}

// TypedLineStat records statistics for a single line typed by a study participant.
//
// If Changes and Duration are both zero, it means the line was a repeat, in which
// case the length will not be 0. But if either Changes or Duration are non-zero, the
// length may be 0, meaning that the user typed some stuff, then backspaced over
// it, then hit return.
type TypedLineStat struct {
	Upn       string
	Completed int64 // Unix time in milliseconds
	Changes   int64
	Length    int64
	Duration  int64
	From      Platform
}

func (s *TypedLineStat) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *TypedLineStat) FromRedis(b []byte) error {
	*s = TypedLineStat{} // clear old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

func (i StudyTypedLineStatsIndex) PushRange(stats []TypedLineStat) error {
	vals := make([]string, 0, len(stats))
	for _, s := range stats {
		v, err := s.ToRedis()
		if err != nil {
			sLog().Error("db failure on typed line stat encode",
				zap.String("studyId", string(i)), zap.Any("stat", s), zap.Error(err))
			return err
		}
		vals = append(vals, string(v))
	}
	if err := platform.PushRange(sCtx(), i, false, vals...); err != nil {
		sLog().Error("db failure on typed line stat push range",
			zap.String("studyId", string(i)), zap.Error(err))
		return err
	}
	return nil
}

func FetchTypedLineStats(studyId, upn string, startDate, endDate int64) ([]TypedLineStat, error) {
	vals, err := platform.FetchRange(sCtx(), StudyTypedLineStatsIndex(studyId+"+"+upn), 0, -1)
	if err != nil {
		return nil, err
	}
	var stat TypedLineStat
	stats := make([]TypedLineStat, 0, len(vals))
	for _, v := range vals {
		if err := stat.FromRedis([]byte(v)); err != nil {
			return nil, err
		}
		if stat.Completed < startDate || stat.Completed > endDate {
			continue
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func FetchAllTypedLineStats(studyId string, start int64, end int64, upns []string) ([][]TypedLineStat, error) {
	var stats [][]TypedLineStat
	if len(upns) > 0 {
		for _, upn := range upns {
			stat, err := FetchTypedLineStats(studyId, upn, start, end)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch line stats for %q: %w", upn, err)
			}
			if len(stat) > 0 {
				stats = append(stats, stat)
			}
		}
	} else {
		upns, err := platform.MapGetKeys(sCtx(), ParticipantIndex(studyId))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch study members: %w", err)
		}
		for _, upn := range upns {
			stat, err := FetchTypedLineStats(studyId, upn, start, end)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch line stats: %w", err)
			}
			if len(stat) == 0 {
				continue
			}
			stats = append(stats, stat)
		}
	}
	return stats, nil
}

// A PhraseStat records the usage of a pre-typed phrase (favorites and repeats).
type PhraseStat struct {
	Hash          string // NV1a hash of lowercased content in base32
	Content       string // Content of canned line
	FavoriteCount int64  // Count of uses as a favorite
	RepeatCount   int64  // Count of uses as a repeat
}

func (s *PhraseStat) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *PhraseStat) FromRedis(b []byte) error {
	*s = PhraseStat{} // clear existing data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

// The PhraseStatsIndex of a study ID maps from phrase hash to PhraseStat.
type PhraseStatsIndex string

func (i PhraseStatsIndex) StoragePrefix() string {
	return "phrase-stats:"
}
func (i PhraseStatsIndex) StorageId() string {
	return string(i)
}

var whitespace = regexp.MustCompile(`\s+`)

func GetOrCreatePhraseStat(studyId, text string) (*PhraseStat, error) {
	text = strings.ToLower(whitespace.ReplaceAllLiteralString(strings.TrimSpace(text), " "))
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(text))
	hash := strconv.FormatUint(hasher.Sum64(), 32)
	val, err := platform.MapGet(sCtx(), PhraseStatsIndex(studyId), hash)
	if err != nil {
		sLog().Error("map get failure on canned line stat lookup",
			zap.String("studyId", studyId), zap.String("hash", hash), zap.Error(err))
		return nil, err
	}
	if val == "" {
		return &PhraseStat{Hash: hash, Content: text}, nil
	}
	var s PhraseStat
	if err := s.FromRedis([]byte(val)); err != nil {
		sLog().Error("deserialization failure on canned line stat",
			zap.String("studyId", studyId), zap.String("hash", hash), zap.Error(err))
		return nil, err
	}
	if s.Content != text {
		sLog().Info("hash collision on canned line stat",
			zap.String("hash", s.Hash),
			zap.String("existing", s.Content), zap.String("ignored", text))
	}
	return &s, nil
}

func SavePhraseStat(studyId string, s *PhraseStat) error {
	b, err := s.ToRedis()
	if err != nil {
		sLog().Error("serialization failure on canned line stat",
			zap.String("studyId", studyId), zap.String("hash", s.Hash), zap.Error(err))
		return err
	}
	if err := platform.MapSet(sCtx(), PhraseStatsIndex(studyId), s.Hash, string(b)); err != nil {
		sLog().Error("db failure on canned line stat save",
			zap.String("studyId", studyId), zap.String("hash", s.Hash), zap.Error(err))
		return err
	}
	return nil
}

func FetchAllPhraseStats(studyId string) ([]PhraseStat, error) {
	m, err := platform.MapGetAll(sCtx(), PhraseStatsIndex(studyId))
	if err != nil {
		return nil, fmt.Errorf("db failure fetch stats map: %w", err)
	}
	result := make([]PhraseStat, 0, len(m))
	for _, v := range m {
		var s PhraseStat
		if err := s.FromRedis([]byte(v)); err != nil {
			return nil, fmt.Errorf("deserialization failure on canned line stat: %w", err)
		}
		result = append(result, s)
	}
	return result, nil
}
