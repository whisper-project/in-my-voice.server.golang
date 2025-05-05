/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"go.uber.org/zap"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type StudyPolicies struct {
	CollectNonStudyStats        bool
	AnonymizeNonStudyLineStats  bool
	SeparateNonStudyRepeatStats bool
}

func (s *StudyPolicies) StoragePrefix() string {
	return "study-policies"
}
func (s *StudyPolicies) StorageId() string {
	return "study-policies"
}
func (s *StudyPolicies) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *StudyPolicies) FromRedis(b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

var (
	DefaultStudyPolicies = StudyPolicies{true, true, true}
	currentStudyPolicies = DefaultStudyPolicies
)

func loadPoliciesConfigAction() {
	ctx := context.Background()
	var s StudyPolicies
	err := platform.LoadObject(ctx, &s)
	if err == nil {
		currentStudyPolicies = s
		return
	}
	// presumably they've never been set, so use the defaults
	if !errors.Is(err, platform.NotFoundError) {
		// warn if we really had a database failure
		sLog().Error("db failure on study policies load", zap.Error(err))
	}
	currentStudyPolicies = DefaultStudyPolicies
	if err = platform.SaveObject(ctx, &currentStudyPolicies); err != nil {
		// warn on database failure
		sLog().Error("db failure on study policies save", zap.Error(err))
	}
}

func init() {
	platform.RegisterForConfigChange("study-policies", loadPoliciesConfigAction)
}

func GetStudyPolicies() StudyPolicies {
	p := currentStudyPolicies
	return p
}

func SetStudyPolicies(p StudyPolicies) {
	currentStudyPolicies = p
	if err := platform.SaveObject(sCtx(), &currentStudyPolicies); err != nil {
		sLog().Error("db failure on study policies save", zap.Error(err))
	}
}

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
	var s StudyParticipant
	var result []*StudyParticipant
	collect := func() error {
		n := s
		result = append(result, &n)
		return nil
	}
	if err := platform.MapObjects(sCtx(), collect, &s); err != nil {
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

type Platform = uint8

const (
	PlatformUnknown = iota
	PlatformPhone
	PlatformTablet
	PlatformComputer
	PlatformBrowser
)

var PlatformNames = []string{"Unknown", "Phone", "Tablet", "Computer", "Browser"}

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

func (t *TypedLineStat) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(t); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (t *TypedLineStat) FromRedis(b []byte) error {
	*t = TypedLineStat{} // clear old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(t)
}

// The TypedLineStatList is keyed by UPN and is the list of all typed lines from that user.
//
// It's a StringList, but each member is a TypedLineStat value.
type TypedLineStatList string

func (t TypedLineStatList) StoragePrefix() string {
	return "typed-line-stat-list:"
}
func (t TypedLineStatList) StorageId() string {
	return string(t)
}

func (t TypedLineStatList) PushRange(stats []TypedLineStat) error {
	vals := make([]string, 0, len(stats))
	for _, s := range stats {
		v, err := s.ToRedis()
		if err != nil {
			sLog().Error("db failure on typed line stat encode",
				zap.String("studyId", string(t)), zap.Any("stat", s), zap.Error(err))
			return err
		}
		vals = append(vals, string(v))
	}
	if err := platform.PushRange(sCtx(), t, false, vals...); err != nil {
		sLog().Error("db failure on typed line stat push range",
			zap.String("studyId", string(t)), zap.Error(err))
		return err
	}
	return nil
}

func FetchTypedLineStats(upn string, startDate, endDate int64) ([]TypedLineStat, error) {
	vals, err := platform.FetchRange(sCtx(), TypedLineStatList(upn), 0, -1)
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

func FetchAllTypedLineStats(start int64, end int64, studyOnly bool, upns []string) ([][]TypedLineStat, error) {
	var stats [][]TypedLineStat
	if len(upns) > 0 {
		for _, upn := range upns {
			stat, err := FetchTypedLineStats(upn, start, end)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch line stats for %q: %w", upn, err)
			}
			if len(stat) > 0 {
				stats = append(stats, stat)
			}
		}
	} else {
		mapper := func(upn string) error {
			if studyOnly && strings.HasPrefix(upn, "NS:") {
				return nil
			}
			stat, err := FetchTypedLineStats(upn, start, end)
			if err != nil {
				return err
			}
			if len(stat) == 0 {
				return nil
			}
			stats = append(stats, stat)
			return nil
		}
		if err := platform.MapKeys(sCtx(), mapper, TypedLineStatList("")); err != nil {
			return nil, fmt.Errorf("failed to fetch line stats: %w", err)
		}
	}
	return stats, nil
}

// CannedLineStat records the usage of pre-typed lines (favorites and repeats).
type CannedLineStat struct {
	Hash          string // NV1a hash of lowercased content in base32
	Content       string // Content of canned line
	FavoriteCount int64  // Count of uses as a favorite
	RepeatCount   int64  // Count of uses as a repeat
}

func (c *CannedLineStat) StoragePrefix() string {
	return "canned-line-stat:"
}
func (c *CannedLineStat) StorageId() string {
	return c.Hash
}
func (c *CannedLineStat) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (c *CannedLineStat) FromRedis(b []byte) error {
	*c = CannedLineStat{} // clear existing data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(c)
}

var whitespace = regexp.MustCompile(`\s+`)

func GetOrCreateCannedLineStat(text string, inStudy bool) (*CannedLineStat, error) {
	text = strings.ToLower(whitespace.ReplaceAllLiteralString(strings.TrimSpace(text), " "))
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(text))
	hash := strconv.FormatUint(hasher.Sum64(), 32)
	if currentStudyPolicies.SeparateNonStudyRepeatStats && !inStudy {
		hash = "NS:" + hash
	}
	c := &CannedLineStat{Hash: hash}
	if err := platform.LoadObject(sCtx(), c); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			return &CannedLineStat{Hash: hash, Content: text}, nil
		}
		sLog().Error("db failure on canned line stat fetch",
			zap.String("hash", hash), zap.Error(err))
		return nil, err
	}
	if c.Content != text {
		sLog().Info("hash collision on canned line stat",
			zap.String("hash", c.Hash),
			zap.String("existing", c.Content), zap.String("ignored", text))
	}
	return c, nil
}

func SaveCannedLineStat(c *CannedLineStat) error {
	if err := platform.SaveObject(sCtx(), c); err != nil {
		sLog().Error("db failure on canned line stat save",
			zap.String("hash", c.Hash), zap.Error(err))
		return err
	}
	return nil
}

func FetchAllCannedLineStats(studyOnly bool) ([]CannedLineStat, error) {
	var result []CannedLineStat
	var s CannedLineStat
	collect := func() error {
		if studyOnly && strings.HasPrefix(s.Hash, "NS:") {
			return nil
		}
		result = append(result, s)
		return nil
	}
	if err := platform.MapObjects(sCtx(), collect, &s); err != nil {
		return nil, err
	}
	return result, nil
}
