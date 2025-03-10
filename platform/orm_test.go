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
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/google/uuid"
)

type OrmTestStruct struct {
	IdField           string    `redis:"id"`
	CreateDate        time.Time `redis:"createDate"`
	CreateDateMillis  int64     `redis:"createDateMillis"`
	CreateDateSeconds float64   `redis:"createDateSeconds"`
	Secret            string    `redis:"secret"`
}

func (data *OrmTestStruct) StoragePrefix() string {
	return "ormTestPrefix:"
}

func (data *OrmTestStruct) StorageId() string {
	if data == nil {
		return ""
	}
	return data.IdField
}

func (data *OrmTestStruct) SetStorageId(id string) error {
	if data == nil {
		return fmt.Errorf("can't set id of nil %T", data)
	}
	data.IdField = id
	return nil
}

func (data *OrmTestStruct) Copy() StructPointer {
	if data == nil {
		return nil
	}
	n := new(OrmTestStruct)
	*n = *data
	return n
}

func (data *OrmTestStruct) Downgrade(in any) (StructPointer, error) {
	if o, ok := in.(OrmTestStruct); ok {
		return &o, nil
	}
	if o, ok := in.(*OrmTestStruct); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not an OrmTestStruct: %#v", in)
}

func TestNilOrmTester(t *testing.T) {
	var data *OrmTestStruct = nil
	if err := LoadFields(context.Background(), data); err == nil {
		t.Errorf("LoadFields on nil pointer didn't fail!")
	}
	if err := SaveFields(context.Background(), data); err == nil {
		t.Errorf("SaveFields on nil pointer didn't fail!")
	}
	if err := MapFields(context.Background(), func() {}, data); err == nil {
		t.Errorf("MapFields on nil pointer didn't fail!")
	}
	if err := DeleteStorage(context.Background(), data); err == nil {
		t.Errorf("DeleteStorage on nil pointer didn't fail!")
	}
}

func TestLoadMissingOrmTester(t *testing.T) {
	data := &OrmTestStruct{IdField: uuid.New().String()}
	if err := LoadFields(context.Background(), data); err == nil {
		t.Errorf("Found stored data for new test object %q", data.IdField)
	} else if !errors.Is(err, StructPointerNotFoundError) {
		t.Errorf("Expected StructPointerNotFound, got %#v", err)
	}
}

func TestSaveLoadDeleteOrmTester(t *testing.T) {
	id := uuid.New().String()
	now := time.Now()
	millis := now.UnixMilli()
	seconds := float64(now.UnixMicro()) / 1_000_000
	saved := OrmTestStruct{IdField: id, CreateDate: now, CreateDateMillis: millis, CreateDateSeconds: seconds, Secret: "shh!"}
	if err := SaveFields(context.Background(), &saved); err != nil {
		t.Errorf("Failed to save stored data for %q: %v", id, err)
	}
	var loaded OrmTestStruct
	_ = loaded.SetStorageId(id)
	if err := LoadFields(context.Background(), &loaded); err != nil {
		t.Errorf("Failed to load stored data for %q: %v", id, err)
	}
	if diff := deep.Equal(saved, loaded); diff != nil {
		t.Errorf("LoadSave data differs: %v", diff)
	}
	if err := DeleteStorage(context.Background(), &loaded); err != nil {
		t.Errorf("Failed to delete stored data for %q: %v", id, err)
	}
	if err := LoadFields(context.Background(), &loaded); err == nil {
		t.Errorf("Succeeded in loading deleted data for %q: %v", id, err)
	}
	if diff := deep.Equal(saved, loaded); diff != nil {
		t.Errorf("Failed load altered fields: %v", diff)
	}
}

func TestSaveMapDeleteOrmTester(t *testing.T) {
	ctx := context.Background()
	id := uuid.New().String()
	now := time.Now()
	millis := now.UnixMilli()
	seconds := float64(now.UnixMicro()) / 1_000_000
	saved := OrmTestStruct{IdField: id, CreateDate: now, CreateDateMillis: millis, CreateDateSeconds: seconds, Secret: id}
	if err := SaveFields(ctx, &saved); err != nil {
		t.Errorf("Failed to save stored data for %q: %v", id, err)
	}
	count := 0
	found := false
	loaded := OrmTestStruct{}
	mapper := func() {
		count++
		if loaded.Secret == id {
			found = true
		}
		if err := DeleteStorage(ctx, &loaded); err != nil {
			t.Errorf("Failed to delete stored data for %q: %v", id, err)
		}
	}
	if err := MapFields(ctx, mapper, &loaded); err != nil {
		t.Fatalf("Failed to map stored data in pass 1: %v", err)
	}
	if !found {
		t.Errorf("Mapped over %#v objects; never found one with secret %q", count, id)
	}
	count = 0
	if err := MapFields(ctx, mapper, &loaded); err != nil {
		t.Errorf("Failed to map stored data in pass 2: %v", err)
	}
	if count != 0 {
		t.Fatalf("Mapped over %#v objects; wanted %#v", count, 0)
	}
}

func TestCopyDowngradeOrmTester(t *testing.T) {
	var orig *OrmTestStruct = nil
	//goland:noinspection GoDfaNilDereference
	copiedOrig := orig.Copy()
	if copiedOrig != nil {
		t.Errorf("copy of nil OrmTestStruct pointer wasn't nil!")
	}
	orig = &OrmTestStruct{IdField: uuid.New().String(), Secret: "shh!"}
	copiedOrig = orig.Copy()
	c, ok := copiedOrig.(*OrmTestStruct)
	if !ok {
		t.Errorf("copy of OrmTestStruct was not a OrmTestStruct: %#v", copiedOrig)
	}
	if diff := deep.Equal(c, copiedOrig); diff != nil {
		t.Errorf("copy of OrmTestStruct differs: %v", diff)
	}
	if c == orig {
		t.Errorf("copy of OrmTestStruct has same address as original: %p, %p", c, orig)
	}
	var template OrmTestStruct
	downgradedCopy, err := template.Downgrade(*c)
	if err != nil {
		t.Errorf("Failed to downgrade OrmTestStruct: %v", err)
	}
	if _, ok := downgradedCopy.(*OrmTestStruct); !ok {
		t.Errorf("Downgraded copy has wrong type: %T", downgradedCopy)
	}
	if diff := deep.Equal(downgradedCopy, orig); diff != nil {
		t.Errorf("Downgraded copy differs from original: %v", diff)
	}
}

var ormTestGob StorableGob = "ormTestGob"

func TestStorableGobInterfaceDefinition(t *testing.T) {
	StorableInterfaceTester(t, ormTestGob, "gob:", "ormTestGob")
}

func TestFetchSetFetchGob(t *testing.T) {
	ctx := context.Background()
	var received map[string][]string
	stored := map[string][]string{"test1": {"test1"}, "test2": {"test2"}}
	if err := FetchGob(ctx, ormTestGob, &received); err == nil {
		t.Errorf("FetchGob of missing gob succeeded (%v), expected failure", received)
	}
	if err := StoreGob(ctx, ormTestGob, &stored); err != nil {
		t.Fatal(err)
	}
	if err := FetchGob(ctx, ormTestGob, &received); err != nil {
		t.Fatal(err)
	}
	if diff := deep.Equal(stored, received); diff != nil {
		t.Error(diff)
	}
	if err := DeleteStorage(ctx, ormTestGob); err != nil {
		t.Error(err)
	}
}

var ormTestString StorableString = "ormTestString"

func TestStorableStringInterfaceDefinition(t *testing.T) {
	StorableInterfaceTester(t, ormTestString, "string:", "ormTestString")
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

var ormTestSet StorableSet = "ormTestSet"

func TestStorableSetInterfaceDefinition(t *testing.T) {
	StorableInterfaceTester(t, ormTestSet, "set:", "ormTestSet")
}

func TestFetchIsNoMembers(t *testing.T) {
	ctx := context.Background()
	if members, err := FetchMembers(ctx, ormTestSet); err != nil || len(members) != 0 {
		t.Errorf("FetchMembers of empty set failed, expected success with no members")
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
	StorableInterfaceTester(t, ormTestSortedSet, "zset:", "ormTestSortedSet")
}

func TestSortedFetchAddFetchRemoveMembers(t *testing.T) {
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
	if err := RemoveMember(ctx, ormTestSortedSet, "a"); err != nil {
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

func TestStorableListInterfaceDefinition(t *testing.T) {
	StorableInterfaceTester(t, ormTestList, "list:", "ormTestList")
}

func TestFetchEmptyRange(t *testing.T) {
	ctx := context.Background()
	if elements, err := FetchRange(ctx, ormTestList, 0, -1); err != nil || len(elements) != 0 {
		t.Errorf("FetchRange of empty list failed, expected success with no elements")
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
		t.Errorf("FetchRange of before list failed, expected success")
	} else if diff := deep.Equal(before, []string{"c", "b", "a", "|", "a", "b", "c"}); diff != nil {
		t.Errorf("FetchRange of before list is:\n%v\nwith differences:\n%v", before, diff)
	}
	if err := RemoveElement(ctx, ormTestList, 0, "b"); err != nil {
		t.Errorf("Failed to remove 'b': %v", err)
	}
	if after, err := FetchRange(ctx, ormTestList, 0, -1); err != nil {
		t.Errorf("FetchRange of after list failed, expected success")
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
		t.Errorf("FetchRange of remaining list failed, expected success")
	} else if diff := deep.Equal(remaining, []string{"c", "a", "b"}); diff != nil {
		t.Errorf("FetchRange of remaining list is:\n%v\ndifferences are:\n%v", remaining, diff)
	}
}

var ormTestMap StorableMap = "ormTestMap"

func TestStorableMapInterfaceDefinition(t *testing.T) {
	StorableInterfaceTester(t, ormTestMap, "map:", "ormTestMap")
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
