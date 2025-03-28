/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/google/uuid"

	"github.com/redis/go-redis/v9"
)

type Storable interface {
	StoragePrefix() string
	StorageId() string
}

// StorableInterfaceTester validates the methods on a Storable type.
// Hand it a value of the type, the expected prefix of the type, and the ID that's in the value.
func StorableInterfaceTester[T Storable](t *testing.T, s T, prefix, id string) {
	t.Helper()
	if s.StoragePrefix() != prefix {
		t.Errorf("(%T).StoragePrefix() returned %q, expected %q", s, s.StoragePrefix(), prefix)
	}
	if v := s.StorageId(); v != id {
		t.Errorf("(%T).StorageId() returned %q. expected %q", s, v, id)
	}
}

func SetExpiration[T Storable](ctx context.Context, obj T, secs int64) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Expire(ctx, key, time.Duration(secs)*time.Second)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func DeleteStorage[T Storable](ctx context.Context, obj T) error {
	if obj.StorageId() == "" {
		return fmt.Errorf("storable has no ID")
	}
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Del(ctx, key)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

type StructPointer interface {
	Storable
	SetStorageId(id string) error
	Copy() StructPointer
	Downgrade(any) (StructPointer, error)
}

// StructPointerInterfaceTester validates the methods on a struct pointer type.
// Hand it a nil of the type, a value of the type, the struct pointed to by that value,
// the expected prefix of the type, and the ID that's in the concrete struct.
func StructPointerInterfaceTester[T StructPointer](t *testing.T, n T, v T, pv any, prefix, id string) {
	t.Helper()
	// check nil behavior
	if n.StoragePrefix() != prefix {
		t.Errorf("nil (%T).StoragePrefix() returned %q, expected %q", n, n.StoragePrefix(), prefix)
	}
	if n.StorageId() != "" {
		t.Errorf("nil (%T).StorageId()returned %q, should return empty string", n, n.StorageId())
	}
	if err := n.SetStorageId("test"); err == nil {
		t.Errorf("nil (%T).SetStorageId() should return an error", n)
	}
	if dup := n.Copy(); dup != nil {
		t.Errorf("nil (%T).Copy() should return nil, got %#v", n, dup)
	}
	if _, err := (v).Downgrade(any(nil)); err == nil {
		t.Errorf("UserProfile.Downgrade(nil) should error out")
	}
	// check value behavior
	if v.StorageId() != id {
		t.Errorf("StorageId is wrong: %s != %s", v.StorageId(), id)
	}
	newId := uuid.NewString()
	if err := v.SetStorageId(newId); err != nil {
		t.Errorf("Failed to set platform id: %v", err)
	}
	if v.StorageId() != newId {
		t.Errorf("StorageId is wrong: %s != %s", v.StorageId(), newId)
	}
	dup := v.Copy()
	if diff := deep.Equal(dup, v); diff != nil {
		t.Error(diff)
	}
	if dg, err := v.Downgrade(any(v)); err != nil {
		t.Error(err)
	} else if diff := deep.Equal(dg, v); diff != nil {
		t.Error(diff)
	}
	if dg, err := v.Downgrade(pv); err != nil {
		t.Error(err)
	} else if dg.StorageId() != id {
		t.Errorf("Downgraded struct ID is wrong: got %s, should be %s", dg.StorageId(), id)
	}
}

type StructPointerNotFound string

func (e StructPointerNotFound) Error() string {
	return fmt.Sprintf("no struct at key: %s", string(e))
}

func (e StructPointerNotFound) Is(err error) bool {
	//goland:noinspection GoTypeAssertionOnErrors
	_, ok := err.(StructPointerNotFound)
	return ok
}

var StructPointerNotFoundError = StructPointerNotFound("")

func LoadFields[T StructPointer](ctx context.Context, obj T) error {
	if obj.StorageId() == "" {
		return fmt.Errorf("storable has no ID")
	}
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HGetAll(ctx, key)
	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to fetch fields of stored object %s: %v", key, err)
	}
	if len(res.Val()) == 0 {
		return StructPointerNotFound(key)
	}
	if err := res.Scan(obj); err != nil {
		return fmt.Errorf("stored object %s cannot be read: %v", key, err)
	}
	return nil
}

func SaveFields[T StructPointer](ctx context.Context, obj T) error {
	if obj.StorageId() == "" {
		return fmt.Errorf("storable has no ID")
	}
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HSet(ctx, key, obj)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func MapFields[T StructPointer](ctx context.Context, f func(), obj T) error {
	if err := obj.SetStorageId(""); err != nil {
		return fmt.Errorf("storable ID cannot be set")
	}
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+obj.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		res := db.HGetAll(ctx, key)
		if err := res.Err(); err != nil {
			return fmt.Errorf("failed to fetch fields of stored object %s: %v", key, err)
		}
		if err := res.Scan(obj); err != nil {
			return fmt.Errorf("stored object %s cannot be read: %v", key, err)
		}
		f()
	}
	if err := iter.Err(); err != nil {
		return err
	}
	return nil
}

type Gob interface {
	Storable
	~string
}

type StorableGob string

func (s StorableGob) StoragePrefix() string {
	return "gob:"
}

func (s StorableGob) StorageId() string {
	return string(s)
}

func FetchGob[T Gob](ctx context.Context, obj T, receiver any) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Get(ctx, key)
	if err := res.Err(); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader([]byte(res.Val()))).Decode(receiver)
}

func StoreGob[T Gob](ctx context.Context, obj T, value any) error {
	if value == nil {
		return fmt.Errorf("cannot store nil value")
	}
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(value); err != nil {
		return err
	}
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Set(ctx, key, b.String(), 0)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

type String interface {
	~string
	Storable
}

type StorableString string

func (s StorableString) StoragePrefix() string {
	return "string:"
}

func (s StorableString) StorageId() string {
	return string(s)
}

func FetchString[T String](ctx context.Context, obj T) (string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Get(ctx, key)
	if err := res.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		} else {
			return "", err
		}
	}
	return res.Val(), nil
}

func StoreString[T String](ctx context.Context, obj T, val string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Set(ctx, key, val, 0)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

type Set interface {
	~string
	Storable
}

type StorableSet string

func (s StorableSet) StoragePrefix() string {
	return "set:"
}

func (s StorableSet) StorageId() string {
	return string(s)
}

func FetchMembers[T Set](ctx context.Context, obj T) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.SMembers(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func IsMember[T Set](ctx context.Context, obj T, member string) (bool, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.SIsMember(ctx, key, member)
	if err := res.Err(); err != nil {
		return false, err
	}
	return res.Val(), nil
}

func AddMembers[T Set](ctx context.Context, obj T, members ...string) error {
	if len(members) == 0 {
		// nothing to add
		return nil
	}
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	args := make([]interface{}, len(members))
	for i, member := range members {
		args[i] = any(member)
	}
	res := db.SAdd(ctx, key, args...)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func RemoveMembers[T Set](ctx context.Context, obj T, members ...string) error {
	if len(members) == 0 {
		// nothing to delete
		return nil
	}
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	args := make([]interface{}, len(members))
	for i, member := range members {
		args[i] = any(member)
	}
	res := db.SRem(ctx, key, args...)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

type SortedSet interface {
	~string
	Storable
}

type StorableSortedSet string

func (s StorableSortedSet) StoragePrefix() string {
	return "zset:"
}

func (s StorableSortedSet) StorageId() string {
	return string(s)
}

func FetchRangeInterval[T SortedSet](ctx context.Context, obj T, start, end int64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZRange(ctx, key, start, end)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func FetchRangeScoreInterval[T SortedSet](ctx context.Context, obj T, min, max float64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	minStr := strconv.FormatFloat(min, 'f', -1, 64)
	maxStr := strconv.FormatFloat(max, 'f', -1, 64)
	res := db.ZRangeByScore(ctx, key, &redis.ZRangeBy{Min: minStr, Max: maxStr})
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func AddScoredMember[T SortedSet](ctx context.Context, obj T, score float64, member string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func RemoveMember[T SortedSet](ctx context.Context, obj T, member string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZRem(ctx, key, member)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

type List interface {
	Storable
	~string
}

type StorableList string

func (s StorableList) StoragePrefix() string {
	return "list:"
}

func (s StorableList) StorageId() string {
	return string(s)
}

func FetchRange[T List](ctx context.Context, obj T, start int64, end int64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.LRange(ctx, key, start, end)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func FetchOneBlocking[T List](ctx context.Context, obj T, onLeft bool, timeout time.Duration) (string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	src, dst := "right", "left"
	if onLeft {
		src, dst = "left", "right"
	}
	res := db.BLMove(ctx, key, key, src, dst, timeout)
	if err := res.Err(); err != nil {
		return "", err
	}
	return res.Val(), nil
}

func MoveOne[T List](ctx context.Context, src T, dst T, srcLeft bool, dstLeft bool) (string, error) {
	db, prefix := GetDb()
	srcKey := prefix + src.StoragePrefix() + src.StorageId()
	dstKey := prefix + dst.StoragePrefix() + dst.StorageId()
	srcSide, dstSide := "right", "right"
	if srcLeft {
		srcSide = "left"
	}
	if dstLeft {
		dstSide = "left"
	}
	res := db.LMove(ctx, srcKey, dstKey, srcSide, dstSide)
	if err := res.Err(); err != nil {
		return "", err
	}
	return res.Val(), nil
}

func PushRange[T List](ctx context.Context, obj T, onLeft bool, members ...string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	args := make([]interface{}, len(members))
	for i, member := range members {
		args[i] = any(member)
	}
	var res *redis.IntCmd
	if onLeft {
		res = db.LPush(ctx, key, args...)
	} else {
		res = db.RPush(ctx, key, args...)
	}
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func RemoveElement[T List](ctx context.Context, obj T, count int64, element string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.LRem(ctx, key, count, any(element))
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

type Map interface {
	Storable
	~string
}

type StorableMap string

func (s StorableMap) StoragePrefix() string {
	return "map:"
}

func (s StorableMap) StorageId() string {
	return string(s)
}

func MapGet[T Map](ctx context.Context, obj T, k string) (string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HGet(ctx, key, k)
	if err := res.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		} else {
			return "", err
		}
	}
	return res.Val(), nil
}

func MapSet[T Map](ctx context.Context, obj T, k string, v string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HSet(ctx, key, k, v)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func MapGetAll[T Map](ctx context.Context, obj T) (map[string]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HGetAll(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func MapRemove[T Map](ctx context.Context, obj T, k string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HDel(ctx, key, k)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}
