/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
	"strings"
	"time"
)

type AdminRole = string

const (
	AdminRoleResearcher         AdminRole = "researcher"
	AdminRoleParticipantManager AdminRole = "participant-manager"
	AdminRoleUserManager        AdminRole = "user-manager"
	AdminRoleSuperAdmin         AdminRole = "super-admin"
)

var allRoles = []AdminRole{AdminRoleResearcher, AdminRoleParticipantManager, AdminRoleUserManager}

type AdminUser struct {
	Id          string
	Email       string
	RoleStorage string
}

func (u *AdminUser) StoragePrefix() string {
	return "admin-user:"
}

func (u *AdminUser) StorageId() string {
	return u.Id
}

func (u *AdminUser) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(u); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (u *AdminUser) FromRedis(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(u)
}

func NewAdminUser(email string) *AdminUser {
	return &AdminUser{Id: uuid.NewString(), Email: email}
}

func (u *AdminUser) HasRole(role AdminRole) bool {
	if u.RoleStorage == AdminRoleSuperAdmin {
		return true
	}
	return strings.Contains(u.RoleStorage, role)
}

func (u *AdminUser) GetRoles() []AdminRole {
	if u.RoleStorage == AdminRoleSuperAdmin {
		return allRoles
	}
	names := strings.Split(u.RoleStorage, ",")
	roles := make([]AdminRole, 0, len(names))
	for _, n := range names {
		roles = append(roles, n)
	}
	return roles
}

func (u *AdminUser) SetRoles(roles []AdminRole) string {
	s := strings.Join(roles, ",")
	if strings.Contains(s, AdminRoleSuperAdmin) {
		return AdminRoleSuperAdmin
	}
	return s
}

func GetAdminUser(id string) (*AdminUser, error) {
	u := &AdminUser{Email: id}
	if err := platform.LoadObject(sCtx(), u); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			return nil, nil
		}
		sLog().Error("db failure on admin user fetch",
			zap.String("id", id), zap.Error(err))
		return nil, err
	}
	return u, nil
}

func SaveAdminUser(u *AdminUser) error {
	if err := platform.SaveObject(sCtx(), u); err != nil {
		sLog().Error("db failure on admin user save",
			zap.String("email", u.Email), zap.Error(err))
		return err
	}
	return nil
}

func DeleteAdminUser(id string) error {
	u := &AdminUser{Email: id}
	if err := platform.DeleteStorage(sCtx(), u); err != nil {
		sLog().Error("db failure on admin user delete",
			zap.String("email", id), zap.Error(err))
		return err
	}
	return nil
}

// A sessionId is an expiring key whose value is the user in the session.
type sessionId string

func (s sessionId) StoragePrefix() string {
	return "session-key:"
}

func (s sessionId) StorageId() string {
	return string(s)
}

func StartSession(roles string) string {
	local, _ := time.LoadLocation("America/Chicago")
	end := time.Now().In(local)
	if end.Hour() >= 4 {
		end.AddDate(0, 0, 1)
	}
	end = time.Date(end.Year(), end.Month(), end.Day(), 4, 0, 0, 0, local)
	id := uuid.NewString()

}
