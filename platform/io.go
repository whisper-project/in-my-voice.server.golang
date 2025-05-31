/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"os"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type (
	ObjectMap       map[string][]any
	StoredObjectMap map[string][]json.RawMessage
)

// DumpObjectsToPath serializes the entire map to the given filepath
func DumpObjectsToPath(what ObjectMap, where string) error {
	file, err := os.OpenFile(where, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := DumpObjectsToStream(what, file); err != nil {
		return err
	}
	return nil
}

// DumpObjectsToStream marshals the objects as JSON to the given stream
func DumpObjectsToStream(what ObjectMap, where io.Writer) error {
	encoder := json.NewEncoder(where)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(what); err != nil {
		return err
	}
	return nil
}

// LoadObjectsFromPath loads the objects dumped to the given filepath.
func LoadObjectsFromPath(where string) (StoredObjectMap, error) {
	file, err := os.OpenFile(where, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return LoadObjectsFromStream(file)
}

// LoadObjectsFromStream creates objects from a stream containing a JSON-serialized object map
func LoadObjectsFromStream(stream io.Reader) (StoredObjectMap, error) {
	decoder := json.NewDecoder(stream)
	var m StoredObjectMap
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func UnmarshalStoredObjects[T any](template T, ms []json.RawMessage) ([]any, error) {
	objs := make([]any, 0, len(ms))
	for _, js := range ms {
		if err := json.Unmarshal(js, &template); err != nil {
			return nil, err
		} else {
			objs = append(objs, template)
		}
	}
	return objs, nil
}

// BOMAwareCSVReader will detect a UTF BOM (Byte Order Mark) at the
// start of the data and transform to UTF8 accordingly.
// If there is no BOM, it will read the data without any transformation.
//
// This code is taken from [this StackOverflow answer](https://stackoverflow.com/a/76023436/558006).
func BOMAwareCSVReader(reader io.Reader) *csv.Reader {
	transformer := unicode.BOMOverride(encoding.Nop.NewDecoder())
	return csv.NewReader(transform.NewReader(reader, transformer))
}
