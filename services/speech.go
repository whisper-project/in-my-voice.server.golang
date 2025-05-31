/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func ElevenParseSettings(settings string) (apiKey, voiceId, voiceName, modelId string, ok bool) {
	var s map[string]any
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return
	}
	key, k := s["apiKey"].(string)
	if !k {
		return
	}
	voice, k := s["voiceId"].(string)
	if !k {
		return
	}
	name, k := s["voiceName"].(string)
	if !k {
		name = ""
	}
	model, k := s["modelId"].(string)
	if !k {
		// older clients don't have a model parameter
		model = ""
	}
	return key, voice, name, model, true
}

func ElevenLabsGenerateSettings(apiKey, voiceId, voiceName, modelId string) string {
	s := map[string]string{"apiKey": apiKey, "voiceId": voiceId, "voiceName": voiceName, "modelId": modelId}
	b, _ := json.Marshal(s)
	return string(b)
}

func ElevenValidateApiKey(apiKey string) (bool, error) {
	uri := "https://api.elevenlabs.io/v1/voices/settings/default"
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("xi-api-key", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// ElevenValidateVoiceId returns ok = true and the voice name if the voiceId is valid
func ElevenValidateVoiceId(apiKey, voiceId string) (name string, ok bool, err error) {
	uri := fmt.Sprintf("https://api.us.elevenlabs.io/v1/voices/%s", voiceId)
	var req *http.Request
	req, err = http.NewRequest("GET", uri, nil)
	if err != nil {
		return
	}
	req.Header.Set("xi-api-key", apiKey)
	client := &http.Client{}
	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var v VoiceInfo
	if err = json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return
	}
	return v.Name, true, nil
}

type VoiceInfo struct {
	VoiceId     string            `json:"voice_id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Labels      map[string]string `json:"labels"`
	Description string            `json:"description"`
	PreviewUrl  string            `json:"preview_url"`
	IsOwner     bool              `json:"is_owner"`
}

type VoiceList struct {
	Voices        []VoiceInfo `json:"voices"`
	HasMore       bool        `json:"has_more"`
	TotalCount    int         `json:"total_count"`
	NextPageToken string      `json:"next_page_token"`
}

func ElevenFetchVoices(apiKey string) ([]VoiceInfo, error) {
	var voices []VoiceInfo
	var nextPageToken string
	hasMore := true
	baseUri := "https://api.us.elevenlabs.io/v2/voices?page_size=100"
	for hasMore {
		uri := baseUri
		if nextPageToken != "" {
			uri += "&page_token=" + nextPageToken
		}
		req, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("xi-api-key", apiKey)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var v VoiceList
		err = json.NewDecoder(resp.Body).Decode(&v)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		voices = append(voices, v.Voices...)
		hasMore = v.HasMore
		nextPageToken = v.NextPageToken
	}
	return voices, nil
}

type ElevenUserAccountInfo struct {
	Tier                           string `json:"tier"`
	CharacterCount                 int64  `json:"character_count"`
	CharacterLimit                 int64  `json:"character_limit"`
	CanExtendCharacterLimit        bool   `json:"can_extend_character_limit"`
	AllowedToExtendCharacterLimit  bool   `json:"allowed_to_extend_character_limit"`
	VoiceSlotsUsed                 int64  `json:"voice_slots_used"`
	ProfessionalVoiceSlotsUsed     int64  `json:"professional_voice_slots_used"`
	VoiceLimit                     int64  `json:"voice_limit"`
	VoiceAddEditCounter            int64  `json:"voice_add_edit_counter"`
	ProfessionalVoiceLimit         int64  `json:"professional_voice_limit"`
	CanExtendVoiceLimit            bool   `json:"can_extend_voice_limit"`
	CanUseInstantVoiceCloning      bool   `json:"can_use_instant_voice_cloning"`
	CanUseProfessionalVoiceCloning bool   `json:"can_use_professional_voice_cloning"`
	Status                         string `json:"status"`
	HasOpenInvoices                bool   `json:"has_open_invoices"`
	MaxCharacterLimitExtension     int64  `json:"max_character_limit_extension"`
	NextCharacterCountResetUnix    int64  `json:"next_character_count_reset_unix"`
	MaxVoiceAddEdits               int64  `json:"max_voice_add_edits"`
	Currency                       string `json:"currency"`
	BillingPeriod                  string `json:"billing_period"`
	CharacterRefreshPeriod         string `json:"character_refresh_period"`
}

var ElevenInvalidApiKeyError = errors.New("invalid ElevenLabs api key")

func ElevenCheckUserAccount(ctx context.Context, apiKey string) (*ElevenUserAccountInfo, error) {
	uri := "https://api.us.elevenlabs.io/v1/user/subscription"
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ElevenInvalidApiKeyError
	default:
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	var info ElevenUserAccountInfo
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
