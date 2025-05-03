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
	"github.com/google/uuid"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
	"strings"
	"time"
)

var AdminTZ = func() *time.Location {
	if loc, err := time.LoadLocation("America/Chicago"); err != nil {
		panic(err)
	} else {
		return loc
	}
}()

type AdminRole = string

const (
	AdminRoleResearcher         AdminRole = "Researcher"
	AdminRoleParticipantManager AdminRole = "Participant Manager"
	AdminRoleUserManager        AdminRole = "User Manager"
	AdminRoleSuperAdmin         AdminRole = "Developer"
)

var (
	RoleLabels = map[AdminRole]string{
		AdminRoleResearcher:         "researcher",
		AdminRoleParticipantManager: "participant",
		AdminRoleUserManager:        "user",
		AdminRoleSuperAdmin:         "developer",
	}
	AllRoles = []AdminRole{AdminRoleResearcher, AdminRoleParticipantManager, AdminRoleUserManager}
)

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
	*u = AdminUser{} // dump old data
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
		return AllRoles
	}
	names := strings.Split(u.RoleStorage, ", ")
	roles := make([]AdminRole, 0, len(names))
	for _, n := range names {
		roles = append(roles, n)
	}
	return roles
}

func (u *AdminUser) SetRoles(roles []AdminRole) {
	s := strings.Join(roles, ", ")
	if strings.Contains(s, AdminRoleSuperAdmin) {
		u.RoleStorage = AdminRoleSuperAdmin
	} else {
		u.RoleStorage = s
	}
}

func GetAdminUser(id string) (*AdminUser, error) {
	u := &AdminUser{Id: id}
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
	u := &AdminUser{Id: id}
	if err := platform.DeleteStorage(sCtx(), u); err != nil {
		sLog().Error("db failure on admin user delete",
			zap.String("email", id), zap.Error(err))
		return err
	}
	return nil
}

func GetAllAdminUsers() ([]*AdminUser, error) {
	u := &AdminUser{}
	var result []*AdminUser
	collect := func() error {
		n := *u
		result = append(result, &n)
		return nil
	}
	if err := platform.MapObjects(sCtx(), collect, u); err != nil {
		sLog().Error("db failure on admin user map", zap.Error(err))
		return nil, err
	}
	return result, nil
}

func LookupAdminUser(email string) (*AdminUser, error) {
	users, err := GetAllAdminUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func EnsureSuperAdmin(email string) error {
	u, err := LookupAdminUser(email)
	if err != nil {
		return err
	}
	if u == nil {
		u = NewAdminUser(email)
		u.SetRoles([]AdminRole{AdminRoleSuperAdmin})
		if err := SaveAdminUser(u); err != nil {
			return err
		}
	}
	return nil
}

// A sessionId is an expiring key whose value is the user id in the session.
type sessionId string

func (s sessionId) StoragePrefix() string {
	return "session-key:"
}

func (s sessionId) StorageId() string {
	return string(s)
}

func StartSession(userId string) (string, error) {
	local, _ := time.LoadLocation("America/Chicago")
	end := time.Now().In(local)
	if end.Hour() >= 4 {
		end = end.AddDate(0, 0, 1)
	}
	end = time.Date(end.Year(), end.Month(), end.Day(), 4, 0, 0, 0, local)
	id := uuid.NewString()
	if err := platform.StoreString(sCtx(), sessionId(id), userId); err != nil {
		sLog().Error("db failure on session start", zap.String("id", id), zap.Error(err))
		return "", err
	}
	if err := platform.SetExpirationAt(sCtx(), sessionId(id), end); err != nil {
		sLog().Error("db failure on session expiration", zap.String("id", id), zap.Error(err))
		return "", err
	}
	sLog().Info("session started",
		zap.String("id", id), zap.String("userId", userId), zap.Time("end", end))
	return id, nil
}

func GetSessionUser(id string) (*AdminUser, error) {
	userId, err := platform.FetchString(sCtx(), sessionId(id))
	if err != nil {
		sLog().Error("db failure on session lookup", zap.String("id", id), zap.Error(err))
		return nil, err
	}
	if userId == "" {
		return nil, nil
	}
	return GetAdminUser(userId)
}

func DeleteSession(id string) error {
	if err := platform.DeleteStorage(sCtx(), sessionId(id)); err != nil {
		sLog().Error("db failure on session delete", zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}
