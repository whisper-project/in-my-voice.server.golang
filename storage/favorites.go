/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
)

type FavoritesSettings struct {
	ProfileId string
	Settings  string
	ETag      string
}

func (f *FavoritesSettings) StoragePrefix() string {
	return "favorites-settings:"
}
func (f *FavoritesSettings) StorageId() string {
	if f == nil {
		return ""
	}
	return f.ProfileId
}
func (f *FavoritesSettings) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(f); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (f *FavoritesSettings) FromRedis(b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(f)
}

func NewFavoritesSettings(profileId, settings string) *FavoritesSettings {
	f := &FavoritesSettings{
		ProfileId: profileId,
		Settings:  settings,
	}
	f.ETag = fmt.Sprintf("%x", md5.Sum([]byte(f.Settings)))
	return f
}

func GetFavoritesSettings(profileId string) (*FavoritesSettings, error) {
	f := &FavoritesSettings{ProfileId: profileId}
	if err := platform.LoadObject(sCtx(), f); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			return nil, nil
		}
		sLog().Error("db failure on settings fetch",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	return f, nil
}

func UpdateFavoritesSettings(profileId, settings string) (bool, error) {
	n := NewFavoritesSettings(profileId, settings)
	o := &FavoritesSettings{ProfileId: profileId}
	if err := platform.LoadObject(sCtx(), o); err != nil {
		if !errors.Is(err, platform.NotFoundError) {
			sLog().Error("db failure on settings update",
				zap.String("profileId", profileId), zap.Error(err))
			return false, err
		}
	}
	if o.ETag == n.ETag {
		return false, nil
	}
	if err := platform.SaveObject(sCtx(), n); err != nil {
		sLog().Error("db failure on settings update",
			zap.String("profileId", profileId), zap.Error(err))
		return false, err
	}
	return true, nil
}
