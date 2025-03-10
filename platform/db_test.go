/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"strings"
	"testing"
)

func TestGetDb(t *testing.T) {
	db0, prefix := GetDb()
	if db0 == nil || !strings.HasSuffix(prefix, ":c:") {
		t.Errorf("initial GetDb didn't return CI db: %v, %q", db0, prefix)
	}
	db1, _ := GetDb()
	if db1 != db0 {
		t.Errorf("GetDb didn't return cached db: %#v, %#v", &db0, &db1)
	}
}

func TestGetMultiDifferentDbs(t *testing.T) {
	dbC, prefixC := GetDb()
	// t.Logf("Initial CI database is: %v, %q", dbC, prefixC)
	if err := PushConfig("development"); err != nil {
		t.Fatalf("failed to push development config: %v", err)
	}
	dbD, prefixD := GetDb()
	if dbC == dbD || prefixC == prefixD {
		t.Fatalf("Dbs before and after dev push are the same: %v & %v, %q & %q", dbC, dbD, prefixC, prefixD)
	}
	if dbD == nil || !strings.HasSuffix(prefixD, ":d:") {
		t.Fatalf("GetDb didn't return dev db after push: %v, %q", dbD, prefixD)
	} else {
		// t.Logf("Pushed dev database is: %v, %q", dbD, prefixD)
	}
	if err := PushConfig("staging"); err != nil {
		t.Fatalf("failed to push staging config: %v", err)
	}
	dbS, prefixS := GetDb()
	if dbD == dbS || prefixD == prefixS {
		t.Fatalf("Dbs before and after stage push are the same: %v & %v, %q & %q", dbD, dbS, prefixD, prefixS)
	}
	if dbS == nil || !strings.HasSuffix(prefixS, ":s:") {
		t.Fatalf("GetDb didn't return staging db after push: %v, %q", dbS, prefixS)
	} else {
		// t.Logf("Pushed staging database is: %v, %q", dbS, prefixS)
	}
	PopConfig()
	dbD2, prefixD2 := GetDb()
	if prefixD2 != prefixD {
		t.Fatalf("Dev prefix after pop is %q", prefixD2)
	}
	if dbD2 == dbD {
		t.Errorf("Dev db before and after pop are the same: %v", dbD)
	}
	if err := PushConfig("production"); err != nil {
		t.Fatalf("failed to push production config: %v", err)
	}
	dbP, prefixP := GetDb()
	if dbP == dbD2 || prefixP == prefixD2 {
		t.Fatalf("Dbs before and after prod push are the same: %v & %v, %q & %q", dbP, dbD2, prefixP, prefixD2)
	}
	if dbP == nil || !strings.HasSuffix(prefixP, ":p:") {
		t.Fatalf("GetDb didn't return prod db after push: %v, %q", dbP, prefixP)
	} else {
		// t.Logf("Pushed prod database is: %v, %q", dbP, prefixP)
	}
	PopConfig()
	dbD3, prefixD3 := GetDb()
	if prefixD3 != prefixD2 {
		t.Fatalf("Dev prefix after pop is %q", prefixD3)
	}
	if dbD3 == dbD2 {
		t.Errorf("Dev db before and after pop are the same: %v", dbD3)
	}
	PopConfig()
	dbT2, prefixT2 := GetDb()
	if prefixT2 != prefixC {
		t.Fatalf("Test prefix after pop is %q", prefixT2)
	}
	if dbT2 == dbC {
		t.Errorf("Test db before and after pop are the same: %v", dbT2)
	}
}
