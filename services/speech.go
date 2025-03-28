/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package services

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func ElevenParseSettings(settings string) (apiKey, voiceId string, ok bool) {
	var s map[string]any
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return "", "", false
	}
	apiKey, aOk := s["apiKey"].(string)
	voiceId, vOk := s["voiceId"].(string)
	return apiKey, voiceId, aOk && vOk
}

func ElevenLabsGenerateSettings(apiKey, voiceId string) string {
	s := map[string]string{"apiKey": apiKey, "voiceId": voiceId}
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

func ElevenValidateVoiceId(apiKey, voiceId string) (bool, error) {
	uri := fmt.Sprintf("https://api.us.elevenlabs.io/v1/voices/%s", voiceId)
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
		defer resp.Body.Close()
		var v VoiceList
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			return nil, err
		}
		voices = append(voices, v.Voices...)
		hasMore = v.HasMore
		nextPageToken = v.NextPageToken
	}
	return voices, nil
}
