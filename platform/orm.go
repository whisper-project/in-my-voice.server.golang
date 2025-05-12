/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// A RedisKey object computes a Redis storage key.
type RedisKey interface {
	StoragePrefix() string
	StorageId() string
}

// RedisKeyTester validates the methods on a RedisKey type.
// Hand it a value of the type, the expected prefix of the type, and the expected ID of the value.
func RedisKeyTester[T RedisKey](t *testing.T, a T, prefix, id string) {
	t.Helper()
	if a.StoragePrefix() != prefix {
		t.Errorf("(%T).StoragePrefix() returned %q, expected %q", a, a.StoragePrefix(), prefix)
	}
	if v := a.StorageId(); v != id {
		t.Errorf("(%T).StorageId() returned %q. expected %q", a, v, id)
	}
}

// A RedisValue object knows how to map to and from stored Redis values.
type RedisValue interface {
	ToRedis() ([]byte, error)
	FromRedis([]byte) error
}

// RedisValueTester validates the methods on a RedisValue type.
// Hand it a concrete value of the type, a second one with a different value,
// and a comparator function for the two values (which will end up the same).
func RedisValueTester[T RedisValue](t *testing.T, v1, v2 T, cmp func(T, T) bool) {
	t.Helper()
	if reflect.ValueOf(v1).Kind() != reflect.Ptr {
		t.Fatalf("RedisValue methods must have pointer receivers; {%T} doesn't", reflect.ValueOf(v1))
	}
	if reflect.ValueOf(v2).Kind() != reflect.Ptr {
		t.Fatalf("RedisValue methods must have pointer receivers; {%T} doesn't", reflect.ValueOf(v2))
	}
	if cmp(v1, v2) {
		t.Fatalf("values must differ to begin with (%v == %v)", v1, v2)
	}
	b, err := v1.ToRedis()
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}
	err = v2.FromRedis(b)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}
	if !cmp(v1, v2) {
		t.Fatalf("values must agree at the end (%v != %v)", v1, v2)
	}
}

func SetExpiration[T RedisKey](ctx context.Context, obj T, secs int64) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Expire(ctx, key, time.Duration(secs)*time.Second)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func SetExpirationAt[T RedisKey](ctx context.Context, obj T, at time.Time) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ExpireAt(ctx, key, at)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func DeleteStorage[T RedisKey](ctx context.Context, obj T) error {
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

func FetchString[T RedisKey](ctx context.Context, obj T) (string, error) {
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

func StoreString[T RedisKey](ctx context.Context, obj T, val string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.Set(ctx, key, val, 0)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func FetchMembers[T RedisKey](ctx context.Context, obj T) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.SMembers(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func IsMember[T RedisKey](ctx context.Context, obj T, member string) (bool, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.SIsMember(ctx, key, member)
	if err := res.Err(); err != nil {
		return false, err
	}
	return res.Val(), nil
}

func AddMembers[T RedisKey](ctx context.Context, obj T, members ...string) error {
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

func RemoveMembers[T RedisKey](ctx context.Context, obj T, members ...string) error {
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

func FetchRangeInterval[T RedisKey](ctx context.Context, obj T, start, end int64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZRange(ctx, key, start, end)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func FetchRangeScoreInterval[T RedisKey](ctx context.Context, obj T, min, max float64) ([]string, error) {
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

func AddScoredMember[T RedisKey](ctx context.Context, obj T, score float64, member string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func RemoveScoredMember[T RedisKey](ctx context.Context, obj T, member string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZRem(ctx, key, member)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func GetMemberScore[T RedisKey](ctx context.Context, obj T, member string) (float64, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.ZScore(ctx, key, member)
	if err := res.Err(); err != nil {
		return 0, err
	}
	return res.Val(), nil
}

func FetchRange[T RedisKey](ctx context.Context, obj T, start int64, end int64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.LRange(ctx, key, start, end)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func FetchOneBlocking[T RedisKey](ctx context.Context, obj T, onLeft bool, timeout time.Duration) (string, error) {
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

func MoveOne[T RedisKey](ctx context.Context, src T, dst T, srcLeft bool, dstLeft bool) (string, error) {
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

func PushRange[T RedisKey](ctx context.Context, obj T, onLeft bool, members ...string) error {
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

func RemoveElement[T RedisKey](ctx context.Context, obj T, count int64, element string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.LRem(ctx, key, count, any(element))
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func MapGet[T RedisKey](ctx context.Context, obj T, k string) (string, error) {
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

func MapSet[T RedisKey](ctx context.Context, obj T, k string, v string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HSet(ctx, key, k, v)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func MapGetKeys[T RedisKey](ctx context.Context, obj T) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HKeys(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func MapGetAll[T RedisKey](ctx context.Context, obj T) (map[string]string, error) {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HGetAll(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

func MapRemove[T RedisKey](ctx context.Context, obj T, k string) error {
	db, prefix := GetDb()
	key := prefix + obj.StoragePrefix() + obj.StorageId()
	res := db.HDel(ctx, key, k)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

var NotFoundError = errors.New("not found")

func LoadValueAtKey[K RedisKey, V RedisValue](ctx context.Context, k K, v V) error {
	db, prefix := GetDb()
	key := prefix + k.StoragePrefix() + k.StorageId()
	bytes, err := db.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return fmt.Errorf("key %v: %w", key, NotFoundError)
	}
	if err != nil {
		return err
	}
	return v.FromRedis(bytes)
}

func SaveValueAtKey[A RedisKey, S RedisValue](ctx context.Context, a A, s S) error {
	db, prefix := GetDb()
	key := prefix + a.StoragePrefix() + a.StorageId()
	bytes, err := s.ToRedis()
	if err != nil {
		return err
	}
	return db.Set(ctx, key, bytes, 0).Err()
}

func MapKeys[K RedisKey](ctx context.Context, f func(string) error, k K) error {
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+k.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		id := strings.TrimPrefix(key, prefix+k.StoragePrefix())
		if err := f(id); err != nil {
			return fmt.Errorf("process key %q, id %q: %w", key, id, err)
		}
	}
	return nil
}

func MapStringsAtKeys[K RedisKey](ctx context.Context, f func(string, string) error, k K) error {
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+k.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		id := strings.TrimPrefix(key, prefix+k.StoragePrefix())
		val, err := db.Get(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("fetch key %q: %w", key, err)
		}
		if err = f(id, val); err != nil {
			return fmt.Errorf("process key %q, id %q, val %q: %w", key, id, val, err)
		}
	}
	return nil
}

func MapValuesAtKeys[K RedisKey, V RedisValue](ctx context.Context, f func() error, k K, v V) error {
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+k.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		bytes, err := db.Get(ctx, key).Bytes()
		if err != nil {
			return fmt.Errorf("fetch key %s: %w", key, err)
		}
		err = v.FromRedis(bytes)
		if err != nil {
			return fmt.Errorf("unmarshal key %s: %w", key, err)
		}
		if err = f(); err != nil {
			return fmt.Errorf("process key %s, value %v: %w", key, v, err)
		}
	}
	return nil
}

type Object interface {
	RedisKey
	RedisValue
}

func LoadObject[T Object](ctx context.Context, obj T) error {
	return LoadValueAtKey(ctx, obj, obj)
}

func SaveObject[T Object](ctx context.Context, obj T) error {
	return SaveValueAtKey(ctx, obj, obj)
}

func MapObjects[T Object](ctx context.Context, f func() error, obj T) error {
	return MapValuesAtKeys(ctx, f, obj, obj)
}

type StorableString string

func (s StorableString) StoragePrefix() string {
	return "string:"
}
func (s StorableString) StorageId() string {
	return string(s)
}

type StorableSet string

func (s StorableSet) StoragePrefix() string {
	return "set:"
}
func (s StorableSet) StorageId() string {
	return string(s)
}

type StorableSortedSet string

func (s StorableSortedSet) StoragePrefix() string {
	return "zset:"
}
func (s StorableSortedSet) StorageId() string {
	return string(s)
}

type StorableList string

func (s StorableList) StoragePrefix() string {
	return "list:"
}
func (s StorableList) StorageId() string {
	return string(s)
}

type StorableMap string

func (s StorableMap) StoragePrefix() string {
	return "map:"
}
func (s StorableMap) StorageId() string {
	return string(s)
}
