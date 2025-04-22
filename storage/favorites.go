/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
)

type FavoritesSettings struct {
	ProfileId string `redis:"profileId"`
	Settings  string `redis:"settings"`
	ETag      string `redis:"eTag"`
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

func (f *FavoritesSettings) SetStorageId(id string) error {
	if f == nil {
		return fmt.Errorf("can't set id of nil %T", f)
	}
	f.ProfileId = id
	return nil
}

func (f *FavoritesSettings) Copy() platform.Object {
	if f == nil {
		return nil
	}
	n := new(FavoritesSettings)
	*n = *f
	return n
}

func (f *FavoritesSettings) Downgrade(a any) (platform.Object, error) {
	if o, ok := a.(FavoritesSettings); ok {
		return &o, nil
	}
	if o, ok := a.(*FavoritesSettings); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", f, a)
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
