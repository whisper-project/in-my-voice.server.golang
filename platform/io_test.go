/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/go-test/deep"
	"golang.org/x/text/encoding/unicode"
)

var (
	objects          = ObjectMap{"string": []any{any("foobar")}, "int64": []any{any(int64(42))}}
	marshaledObjects []byte
	loadedObjects    StoredObjectMap
)

func init() {
	b, err := json.MarshalIndent(objects, "", "  ")
	if err != nil {
		panic(err)
	}
	marshaledObjects = append(b, []byte("\n")...)
	err = json.Unmarshal(marshaledObjects, &loadedObjects)
	if err != nil {
		panic(err)
	}
}

func TestDumpLoadMarshalObjects(t *testing.T) {
	f, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	defer os.Remove(path)
	f.Close()
	err = DumpObjectsToPath(objects, path)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if s1, s2 := string(marshaledObjects), string(b); s1 != s2 {
		t.Errorf("expected: %q, got: %q", s1, s2)
	}
	lMap, err := LoadObjectsFromPath(path)
	if err != nil {
		t.Fatal(err)
	} else if diff := deep.Equal(loadedObjects, lMap); diff != nil {
		t.Error(diff)
	}
	ss, err := UnmarshalStoredObjects("", lMap["string"])
	if err != nil {
		t.Fatal(err)
	}
	is, err := UnmarshalStoredObjects(int64(0), lMap["int64"])
	if err != nil {
		t.Fatal(err)
	}
	oMap := ObjectMap{"string": ss, "int64": is}
	if diff := deep.Equal(objects, oMap); diff != nil {
		t.Error(diff)
	}
}

func TestBOMAwareCSVReader(t *testing.T) {
	s := "世界"
	u8 := "世界\n"
	u8Bytes := []byte(u8)
	reader := BOMAwareCSVReader(bytes.NewReader(u8Bytes))
	row, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != s {
		t.Errorf("col should be %q but is %q", s, row[0])
	}
	u8Bom := append([]byte{0xEF, 0xBB, 0xBF}, u8Bytes...)
	reader = BOMAwareCSVReader(bytes.NewReader(u8Bom))
	row, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != s {
		t.Errorf("col should be %q but is %q", s, row[0])
	}
	enc := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	b, err := enc.Bytes(u8Bom)
	if err != nil {
		t.Fatal(err)
	}
	reader = BOMAwareCSVReader(bytes.NewReader(b))
	row, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != s {
		t.Errorf("col should be %q but is %q", s, row[0])
	}
	enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
	b, err = enc.Bytes(u8Bom)
	if err != nil {
		t.Fatal(err)
	}
	reader = BOMAwareCSVReader(bytes.NewReader(b))
	row, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != s {
		t.Errorf("col should be %q but is %q", s, row[0])
	}
}
