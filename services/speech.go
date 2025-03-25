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
