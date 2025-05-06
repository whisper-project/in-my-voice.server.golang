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
	"github.com/google/uuid"
	"slices"
	"testing"
	"time"

	"github.com/go-test/deep"
)

var ormTestString StorableString = "ormTestString"

func TestStorableStringInterfaceDefinition(t *testing.T) {
	RedisKeyTester(t, ormTestString, "string:", "ormTestString")
}

func TestFetchSetFetchString(t *testing.T) {
	ctx := context.Background()
	if val, err := FetchString(ctx, ormTestString); err != nil || val != "" {
		t.Errorf("FetchString of missing string failed (%v), expected success with empty value (%s)", err, val)
	}
	if err := StoreString(ctx, ormTestString, string(ormTestString)); err != nil {
		t.Error(err)
	}
	if val, err := FetchString(ctx, ormTestString); err != nil || val != string(ormTestString) {
		t.Errorf("FetchString of failed (%v), expected %q got %q", err, string(ormTestString), val)
	}
	if err := DeleteStorage(ctx, ormTestString); err != nil {
		t.Error(err)
	}
}

func TestExpireString(t *testing.T) {
	ctx := context.Background()
	if err := StoreString(ctx, ormTestString, string(ormTestString)); err != nil {
		t.Fatal(err)
	}
	if err := SetExpiration(ctx, ormTestString, 1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if val, err := FetchString(ctx, ormTestString); err == nil && val != "" {
		t.Errorf("FetchString of expired string failed (%v), expected success with empty value (%s)", err, val)
	}
}

func TestExpireAtString(t *testing.T) {
	ctx := context.Background()
	if err := StoreString(ctx, ormTestString, string(ormTestString)); err != nil {
		t.Fatal(err)
	}
	if err := SetExpirationAt(ctx, ormTestString, time.Now().Add(1*time.Second)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if val, err := FetchString(ctx, ormTestString); err == nil && val != "" {
		t.Errorf("FetchString of expired string failed (%v), expected success with empty value (%s)", err, val)
	}
}

var ormTestSet StorableSet = "ormTestSet"

func TestStorableSetInterfaceDefinition(t *testing.T) {
	RedisKeyTester(t, ormTestSet, "set:", "ormTestSet")
}

func TestFetchIsNoMembers(t *testing.T) {
	ctx := context.Background()
	if members, err := FetchMembers(ctx, ormTestSet); err != nil || len(members) != 0 {
		t.Errorf("FetchMembers of the empty set failed, expected success with no members")
	}
	if val, err := IsMember(ctx, ormTestSet, "b"); err != nil {
		t.Errorf("IsMember failed: %v", err)
	} else if val {
		t.Errorf("IsMember returned true, expected false")
	}
}

func TestAddFetchIsRemoveMembers(t *testing.T) {
	ctx := context.Background()
	saved := []string{"a", "b", "c", "b", "a"}
	if err := AddMembers(ctx, ormTestSet, saved...); err != nil {
		t.Errorf("Failed to add saved: %v", err)
	}
	if err := AddMembers(ctx, ormTestSet); err != nil {
		t.Errorf("Failed to add empty: %v", err)
	}
	if found, err := FetchMembers(ctx, ormTestSet); err != nil {
		t.Errorf("FetchMembers failed: %v", err)
	} else if len(found) != 3 {
		t.Errorf("FetchMembers returned %d results, expected 3: %#v", len(found), found)
	}
	if val, err := IsMember(ctx, ormTestSet, "b"); err != nil {
		t.Errorf("IsMember failed: %v", err)
	} else if !val {
		t.Errorf("IsMember returned false, expected true")
	}
	if err := RemoveMembers(ctx, ormTestSet, "b", "c"); err != nil {
		t.Errorf("Failed to remove members: %v", err)
	}
	if err := RemoveMembers(ctx, ormTestSet); err != nil {
		t.Errorf("Failed to remove empty: %v", err)
	}
	if found, err := FetchMembers(ctx, ormTestSet); err != nil {
		t.Errorf("FetchMembers failed: %v", err)
	} else if len(found) != 1 {
		t.Errorf("FetchMembers returned %d results, expected 1: %#v", len(found), found)
	}
	if err := DeleteStorage(ctx, ormTestSet); err != nil {
		t.Errorf("Failed to delete stored data for %q: %v", ormTestSet, err)
	}
}

var ormTestSortedSet StorableSortedSet = "ormTestSortedSet"

func TestStorableSortedSetInterfaceDefinition(t *testing.T) {
	RedisKeyTester(t, ormTestSortedSet, "zset:", "ormTestSortedSet")
}

func TestSortedFetchAddScoreFetchRemoveMember(t *testing.T) {
	ctx := context.Background()
	sorted := []string{"a", "b", "c"}
	if members, err := FetchRangeInterval(ctx, ormTestSortedSet, 0, -1); err != nil || len(members) != 0 {
		t.Errorf("FetchRangeInterval of empty failed (%v) or has members: %v", err, members)
	}
	if err := AddScoredMember(ctx, ormTestSortedSet, 3, "c"); err != nil {
		t.Error(err)
	}
	if err := AddScoredMember(ctx, ormTestSortedSet, 2, "b"); err != nil {
		t.Error(err)
	}
	if err := AddScoredMember(ctx, ormTestSortedSet, 1, "a"); err != nil {
		t.Error(err)
	}
	if score, err := GetMemberScore(ctx, ormTestSortedSet, "c"); err != nil {
		t.Error(err)
	} else if score != 3 {
		t.Errorf("GetMemberScore returned %v, expected %v", score, 3)
	}
	if found, err := FetchRangeInterval(ctx, ormTestSortedSet, 0, -1); err != nil {
		t.Error(err)
	} else if diff := deep.Equal(sorted, found); diff != nil {
		t.Error(diff)
	}
	if found, err := FetchRangeScoreInterval(ctx, ormTestSortedSet, 2, 3); err != nil {
		t.Error(err)
	} else if diff := deep.Equal(sorted[1:3], found); diff != nil {
		t.Error(diff)
	}
	if err := RemoveScoredMember(ctx, ormTestSortedSet, "a"); err != nil {
		t.Error(err)
	}
	if found, err := FetchRangeInterval(ctx, ormTestSortedSet, 0, -1); err != nil {
		t.Error(err)
	} else if diff := deep.Equal(sorted[1:3], found); diff != nil {
		t.Error(diff)
	}
	if err := DeleteStorage(ctx, ormTestSortedSet); err != nil {
		t.Error(err)
	}
}

var ormTestList StorableList = "ormTestList"
var ormTestList2 StorableList = "ormTestList2"

func TestStorableListInterfaceDefinition(t *testing.T) {
	RedisKeyTester(t, ormTestList, "list:", "ormTestList")
}

func TestFetchEmptyRange(t *testing.T) {
	ctx := context.Background()
	if elements, err := FetchRange(ctx, ormTestList, 0, -1); err != nil || len(elements) != 0 {
		t.Errorf("FetchRange of an empty list failed, expected success with no elements")
	}
}

func TestAddFetchRemoveRange(t *testing.T) {
	ctx := context.Background()
	if err := PushRange(ctx, ormTestList, true, "|"); err != nil {
		t.Errorf("Failed to push center: %v", err)
	}
	if err := PushRange(ctx, ormTestList, true, "a", "b", "c"); err != nil {
		t.Errorf("Failed to push left: %v", err)
	}
	if err := PushRange(ctx, ormTestList, false, "a", "b", "c"); err != nil {
		t.Errorf("Failed to push right: %v", err)
	}
	if before, err := FetchRange(ctx, ormTestList, 0, -1); err != nil {
		t.Errorf("FetchRange of the before list failed, expected success")
	} else if diff := deep.Equal(before, []string{"c", "b", "a", "|", "a", "b", "c"}); diff != nil {
		t.Errorf("FetchRange of before list is:\n%v\nwith differences:\n%v", before, diff)
	}
	if err := RemoveElement(ctx, ormTestList, 0, "b"); err != nil {
		t.Errorf("Failed to remove 'b': %v", err)
	}
	if after, err := FetchRange(ctx, ormTestList, 0, -1); err != nil {
		t.Errorf("FetchRange of the after list failed, expected success")
	} else if diff := deep.Equal(after, []string{"c", "a", "|", "a", "c"}); diff != nil {
		t.Errorf("FetchRange of after list is:\n%v\nwith differences:\n%v", after, diff)
	}
	if err := DeleteStorage(ctx, ormTestList); err != nil {
		t.Errorf("Failed to delete stored data for %q: %v", ormTestList, err)
	}
}

func TestFetchOneBlocking(t *testing.T) {
	ctx := context.Background()
	defer func() {
		if err := DeleteStorage(ctx, ormTestList); err != nil {
			t.Errorf("Failed to delete stored data for %q: %v", ormTestList, err)
		}
	}()
	c := make(chan string)
	go func() {
		if element, err := FetchOneBlocking(ctx, ormTestList, false, 2*time.Second); err != nil {
			t.Errorf("FetchOneBlocking failed: %v", err)
			c <- "failed"
		} else {
			c <- element
		}
	}()
	time.Sleep(500 * time.Millisecond)
	if err := PushRange(ctx, ormTestList, false, "a", "b", "c"); err != nil {
		t.Fatalf("Failed to push right: %v", err)
	}
	received := <-c
	if received != "c" {
		t.Errorf("FetchOneBlocking got %q", received)
	}
	if remaining, err := FetchRange(ctx, ormTestList, 0, -1); err != nil {
		t.Errorf("FetchRange of the remaining list failed, expected success")
	} else if diff := deep.Equal(remaining, []string{"c", "a", "b"}); diff != nil {
		t.Errorf("FetchRange of remaining list is:\n%v\ndifferences are:\n%v", remaining, diff)
	}
}

func TestMoveRange(t *testing.T) {
	ctx := context.Background()
	defer func() {
		if err := DeleteStorage(ctx, ormTestList); err != nil {
			t.Errorf("Failed to delete stored data for %q: %v", ormTestList, err)
		}
		if err := DeleteStorage(ctx, ormTestList2); err != nil {
			t.Errorf("Failed to delete stored data for %q: %v", ormTestList2, err)
		}
	}()
	if _, err := MoveOne(ctx, ormTestList, ormTestList2, false, true); err == nil {
		t.Errorf("Move one on an empty source should fail")
	}
	if err := PushRange(ctx, ormTestList, false, "a", "b", "c"); err != nil {
		t.Fatalf("Failed to push right: %v", err)
	}
	if _, err := MoveOne(ctx, ormTestList, ormTestList2, false, true); err != nil {
		t.Errorf("Move one failed: %v", err)
	}
	if remaining, err := FetchRange(ctx, ormTestList, 0, -1); err != nil {
		t.Errorf("FetchRange of the source list failed, expected success")
	} else if diff := deep.Equal(remaining, []string{"a", "b"}); diff != nil {
		t.Errorf("FetchRange of source list is:\n%v\ndifferences are:\n%v", remaining, diff)
	}
	if _, err := MoveOne(ctx, ormTestList, ormTestList2, false, true); err != nil {
		t.Errorf("Move one failed: %v", err)
	}
	if remaining, err := FetchRange(ctx, ormTestList, 0, -1); err != nil {
		t.Errorf("FetchRange of the source list failed, expected success")
	} else if diff := deep.Equal(remaining, []string{"a"}); diff != nil {
		t.Errorf("FetchRange of source list is:\n%v\ndifferences are:\n%v", remaining, diff)
	}
	if remaining, err := FetchRange(ctx, ormTestList2, 0, -1); err != nil {
		t.Errorf("FetchRange of the destination list failed, expected success")
	} else if diff := deep.Equal(remaining, []string{"b", "c"}); diff != nil {
		t.Errorf("FetchRange of destination list is:\n%v\ndifferences are:\n%v", remaining, diff)
	}
}

var ormTestMap StorableMap = "ormTestMap"

func TestStorableMapInterfaceDefinition(t *testing.T) {
	RedisKeyTester(t, ormTestMap, "map:", "ormTestMap")
}

func TestOrmTestMap(t *testing.T) {
	ctx := context.Background()
	// Attempt to fetch an element that doesn't exist
	if val, err := MapGet(ctx, ormTestMap, "nonexistent"); err != nil || val != "" {
		t.Errorf("FetchMapElement of nonexistent key failed (%v), expected success with empty value (%q)", err, val)
	}

	// Add an element to the map
	key, value := "testKey", "testValue"
	if err := MapSet(ctx, ormTestMap, key, value); err != nil {
		t.Errorf("SetMapElement failed: %v", err)
	}

	// Retrieve the element and test its value
	if val, err := MapGet(ctx, ormTestMap, key); err != nil || val != value {
		t.Errorf("FetchMapElement failed (%v), expected %q but got %q", err, value, val)
	}

	// Add another element to the map
	anotherKey, anotherValue := "anotherKey", "anotherValue"
	if err := MapSet(ctx, ormTestMap, anotherKey, anotherValue); err != nil {
		t.Errorf("SetMapElement failed: %v", err)
	}

	// Retrieve all elements and validate their values
	if allElements, err := MapGetAll(ctx, ormTestMap); err != nil {
		t.Errorf("MapGetAll failed: %v", err)
	} else if len(allElements) != 2 || allElements[key] != value || allElements[anotherKey] != anotherValue {
		t.Errorf("MapGetAll returned unexpected results: %v", allElements)
	}

	// Remove an element from the map
	if err := MapRemove(ctx, ormTestMap, key); err != nil {
		t.Errorf("MapRemove failed: %v", err)
	}

	// Verify the removed key does not exist
	if val, err := MapGet(ctx, ormTestMap, key); err != nil || val != "" {
		t.Errorf("MapGet after removal failed (%v), expected empty value but got %q", err, val)
	}

	// Clean up by deleting the map
	if err := DeleteStorage(ctx, &ormTestMap); err != nil {
		t.Errorf("Failed to delete stored map for %q: %v", ormTestMap, err)
	}
}

type OrmTestStruct struct {
	IdField           string
	CreateDate        time.Time
	CreateDateMillis  int64
	CreateDateSeconds float64
	Secret            string
}

func (data *OrmTestStruct) StoragePrefix() string {
	return "ormTestPrefix:"
}

func (data *OrmTestStruct) StorageId() string {
	return data.IdField
}

func (data *OrmTestStruct) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(data); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (data *OrmTestStruct) FromRedis(b []byte) error {
	*data = OrmTestStruct{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(data)
}

func TestOrmTesterInterfaceDefinition(t *testing.T) {
	v1 := OrmTestStruct{IdField: "v1", CreateDate: time.Now()}
	v2 := OrmTestStruct{IdField: "v2"}
	RedisKeyTester(t, &v1, "ormTestPrefix:", "v1")
	RedisValueTester(t, &v1, &v2, func(l, r *OrmTestStruct) bool {
		return l.IdField == r.IdField && l.CreateDate.Equal(r.CreateDate)
	})
}

func TestLoadMissingOrmTester(t *testing.T) {
	data := &OrmTestStruct{IdField: uuid.New().String()}
	if err := LoadObject(context.Background(), data); err == nil {
		t.Fatalf("no error fetching new uuid key %q", data.IdField)
	} else if !errors.Is(err, NotFoundError) {
		t.Errorf("Expected NotFound, got %#v", err)
	}
}

func TestSaveLoadDeleteOrmTester(t *testing.T) {
	id := uuid.New().String()
	now := time.Now()
	millis := now.UnixMilli()
	seconds := float64(now.UnixMicro()) / 1_000_000
	saved := OrmTestStruct{IdField: id, CreateDate: now, CreateDateMillis: millis, CreateDateSeconds: seconds, Secret: "shh!"}
	if err := SaveObject(context.Background(), &saved); err != nil {
		t.Fatal(err)
	}
	loaded := OrmTestStruct{IdField: id}
	if err := LoadObject(context.Background(), &loaded); err != nil {
		t.Fatal(err)
	}
	if diff := deep.Equal(saved, loaded); diff != nil {
		t.Errorf("LoadSave data differs: %v", diff)
	}
	if err := DeleteStorage(context.Background(), &loaded); err != nil {
		t.Fatal(err)
	}
	if err := LoadObject(context.Background(), &loaded); err == nil {
		t.Fatalf("Succeeded in loading deleted data for %q", id)
	}
	if diff := deep.Equal(saved, loaded); diff != nil {
		t.Errorf("Failed load altered fields: %v", diff)
	}
}

func TestSaveMapDeleteOrmTester(t *testing.T) {
	ctx := context.Background()
	id1 := uuid.New().String() + "-id1"
	id2 := uuid.New().String() + "-id2"
	saved1 := OrmTestStruct{IdField: id1, CreateDateMillis: 1000}
	if err := SaveObject(ctx, &saved1); err != nil {
		t.Errorf("Failed to save stored data for %q: %v", id1, err)
	}
	saved2 := OrmTestStruct{IdField: id2, Secret: "secret2"}
	if err := SaveObject(ctx, &saved2); err != nil {
		t.Errorf("Failed to save stored data for %q: %v", id2, err)
	}
	count := 0
	found1 := false
	found2 := false
	loaded := OrmTestStruct{IdField: "value doesn't matter"}
	mapper := func() error {
		count++
		if loaded.IdField == id1 && loaded.CreateDateMillis == 1000 {
			found1 = true
			if loaded.Secret != "" {
				return fmt.Errorf("loaded data for %q has secret %q, expected %q", id1, loaded.Secret, "")
			}
		}
		if loaded.IdField == id2 && loaded.Secret == "secret2" {
			found2 = true
			if loaded.CreateDateMillis != 0 {
				return fmt.Errorf("loaded data for %q has millis %d, expected %v", id1, loaded.CreateDateMillis, 0)
			}
		}
		if err := DeleteStorage(ctx, &loaded); err != nil {
			return fmt.Errorf("lailed to delete stored data for %q: %v", loaded.IdField, err)
		}
		return nil
	}
	if err := MapObjects(ctx, mapper, &loaded); err != nil {
		t.Fatalf("Failed to map stored data in pass 1: %v", err)
	}
	if count < 2 {
		t.Logf("Mapped over %#v OrmTester objects; expected at least 2", count)
	}
	if !found1 || !found2 {
		t.Errorf("Mapped over %#v objects; found1 = %v; found2 = %v", count, found1, found2)
	}
	count = 0
	if err := MapObjects(ctx, mapper, &loaded); err != nil {
		t.Errorf("Failed to map stored data in pass 2: %v", err)
	}
	if count != 0 {
		t.Fatalf("Mapped over %#v objects; wanted %#v", count, 0)
	}
}

type ormTestKey string

func (o ormTestKey) StoragePrefix() string {
	return "test-key-prefix:"
}
func (o ormTestKey) StorageId() string {
	return string(o)
}

func TestMapKeys(t *testing.T) {
	ctx := context.Background()
	key1 := uuid.NewString()
	if err := StoreString(ctx, ormTestKey(key1), "value1"); err != nil {
		t.Fatal(err)
	}
	key2 := uuid.NewString()
	if err := StoreString(ctx, ormTestKey(key2), "value2"); err != nil {
		t.Fatal(err)
	}
	key3 := uuid.NewString()
	if err := StoreString(ctx, ormTestKey(key3), "value3"); err != nil {
		t.Fatal(err)
	}
	var foundKeys []string
	mapper1 := func(k string) error {
		foundKeys = append(foundKeys, k)
		if err := DeleteStorage(ctx, ormTestKey(k)); err != nil {
			return err
		}
		return nil
	}
	if err := MapKeys(ctx, mapper1, ormTestKey("value doesn't matter")); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(foundKeys, key1) && !slices.Contains(foundKeys, key2) && !slices.Contains(foundKeys, key3) {
		t.Errorf("found keys %v; at least one is missing", foundKeys)
	}
	mapper2 := func(k string) error {
		return fmt.Errorf("found key %q; expected none", k)
	}
	if err := MapKeys(ctx, mapper2, ormTestKey("")); err != nil {
		t.Fatal(err)
	}
}
