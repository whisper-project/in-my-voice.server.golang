/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"context"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"go.uber.org/zap"
	"time"
)

var (
	speechMonitors platform.StorableSortedSet = "speech-monitors"
	maxCharsPerDay int64                      = 60 * 5 * 30 * 12 // 60 wpm at 5 chars/word for 30 mins half a day
	maxRateDelay   int64                      = 7 * 24 * 60 * 60 // 1 week
)

// EnsureMonitor makes sure that the given profile is having its ElevenLabs account
// checked for character limits.
func EnsureMonitor(profileId, apiKey string) error {
	if _, err := platform.GetMemberScore(sCtx(), speechMonitors, profileId); err == nil {
		return nil
	}
	s := NewSpeechMonitor(profileId, apiKey)
	if err := platform.SaveObject(sCtx(), s); err != nil {
		sLog().Error("Failed to db of new monitor",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	if err := platform.AddScoredMember(sCtx(), speechMonitors, 0, profileId); err != nil {
		sLog().Error("Failed to add new monitor",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	sLog().Info("Added new monitor", zap.String("profileId", profileId))
	return nil
}

// RemoveMonitor should be done on every profile that loses its ElevenLabs API key
func RemoveMonitor(profileId string) error {
	if err := platform.RemoveScoredMember(sCtx(), speechMonitors, profileId); err != nil {
		sLog().Error("Failed to remove scored monitor",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	s := SpeechMonitor{ProfileId: profileId}
	if err := platform.DeleteStorage(sCtx(), &s); err != nil {
		sLog().Error("Failed to remove monitor",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	sLog().Info("Removed monitor", zap.String("profileId", profileId))
	return nil
}

func FetchMonitorsForUpdate(ctx context.Context) ([]*SpeechMonitor, error) {
	now := float64(time.Now().Unix())
	profiles, err := platform.FetchRangeScoreInterval(ctx, speechMonitors, -1, now)
	if err != nil {
		sLog().Error("Failed to fetch monitors for update", zap.Error(err))
		return nil, err
	}
	monitors := make([]*SpeechMonitor, 0, len(profiles))
	for _, profile := range profiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			s := SpeechMonitor{ProfileId: profile}
			if err := platform.LoadObject(ctx, &s); err != nil {
				sLog().Error("Failed to db of monitor",
					zap.String("profileId", profile), zap.Error(err))
				return nil, err
			}
			monitors = append(monitors, &s)
		}
	}
	return monitors, nil
}

type SpeechMonitor struct {
	ProfileId  string `redis:"profileId"`
	ApiKey     string `redis:"apiKey"`
	UsedChars  int64  `redis:"usedChars"`
	LimitChars int64  `redis:"limitChars"`
	NextCheck  int64  `redis:"nextCheck"`
	NextRenew  int64  `redis:"nextRenew"`
}

func (s *SpeechMonitor) StoragePrefix() string {
	return "speech-monitor:"
}

func (s *SpeechMonitor) StorageId() string {
	if s == nil {
		return ""
	}
	return s.ProfileId
}

func (s *SpeechMonitor) SetStorageId(id string) error {
	if s == nil {
		return fmt.Errorf("can't set id of nil %T", s)
	}
	s.ProfileId = id
	return nil
}

func (s *SpeechMonitor) Copy() platform.Object {
	if s == nil {
		return nil
	}
	n := new(SpeechMonitor)
	*n = *s
	return n
}

func (s *SpeechMonitor) Downgrade(a any) (platform.Object, error) {
	if o, ok := a.(SpeechMonitor); ok {
		return &o, nil
	}
	if o, ok := a.(*SpeechMonitor); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", s, a)
}

func NewSpeechMonitor(profileId, apiKey string) *SpeechMonitor {
	return &SpeechMonitor{ProfileId: profileId, ApiKey: apiKey}
}

func (s *SpeechMonitor) Update(ctx context.Context) error {
	var lastPct int64
	if s.LimitChars > 0 {
		lastPct = s.UsedChars * 100 / s.LimitChars
	}
	lastRenew := s.NextRenew
	info, err := services.ElevenCheckUserAccount(ctx, s.ApiKey)
	if err != nil {
		sLog().Error("Eleven Labs user account check failed",
			zap.String("profileId", s.ProfileId), zap.Error(err))
		return err
	}
	s.UsedChars = info.CharacterCount
	s.LimitChars = info.CharacterLimit
	s.NextRenew = info.NextCharacterCountResetUnix
	curPct := s.UsedChars * 100 / s.LimitChars
	rateDelay := min(((s.LimitChars-s.UsedChars)*24*3600)/maxCharsPerDay, maxRateDelay)
	s.NextCheck = min(time.Now().Unix()+rateDelay, s.NextRenew)
	switch {
	case lastRenew < s.NextRenew && lastPct >= 99:
		// we've renewed and the person had been cut off, re-enable them
		_ = ProfileUsageDidUpdate(s.ProfileId)
	case curPct >= 99 && lastPct < 99:
		// user has hit their quota, notify them
		_ = ProfileUsageDidUpdate(s.ProfileId)
		// don't check again until nextRenew
		s.NextCheck = s.NextRenew
	case curPct >= 90 && lastPct < 90:
		// user just got to 90%, warn them
		_ = ProfileUsageDidUpdate(s.ProfileId)
		fallthrough
	case curPct >= 90:
		// user is approaching their cutoff, check every hour
		s.NextRenew = time.Now().Unix() + 3500
	}
	sLog().Info("Completed monitor update",
		zap.String("profileId", s.ProfileId), zap.Int64("pctUsed", curPct),
		zap.Int64("usedChars", s.UsedChars), zap.Int64("limitChars", s.LimitChars),
		zap.Time("nextCheck", time.Unix(s.NextCheck, 0)))
	return nil
}

type NotifiedUsageClients string

func (n NotifiedUsageClients) StoragePrefix() string {
	return "notified-usage-clients:"
}

func (n NotifiedUsageClients) StorageId() string {
	return string(n)
}

func ProfileUsageDidUpdate(profileId string) error {
	n := NotifiedUsageClients(profileId)
	if err := platform.DeleteStorage(sCtx(), n); err != nil {
		sLog().Error("delete storage failure", zap.Error(err))
		return err
	}
	if err := platform.AddMembers(sCtx(), n, "none"); err != nil {
		sLog().Error("add set member failed", zap.Error(err))
		return err
	}
	return nil
}

func ProfileClientUsageNeedsNotification(profileId, clientId string) (bool, error) {
	n := NotifiedUsageClients(profileId)
	isMember, err := platform.IsMember(sCtx(), n, clientId)
	if err != nil {
		sLog().Error("lookup set member failed", zap.Error(err))
		return false, err
	}
	return !isMember, nil
}

func ProfileClientUsageWasNotified(profileId, clientId string) error {
	n := NotifiedUsageClients(profileId)
	if err := platform.AddMembers(sCtx(), n, clientId); err != nil {
		sLog().Error("add set member failed", zap.Error(err))
		return err
	}
	return nil
}
