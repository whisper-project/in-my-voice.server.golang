/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"testing"
)

func TestSetIfMissing(t *testing.T) {
	var i1 int64
	i2 := int64(3)
	var s1 string
	s2 := "3"
	SetIfMissing(&i1, 1)
	if i1 != 1 {
		t.Errorf("i1 should be 1 but is %d", i1)
	}
	SetIfMissing(&i2, 2)
	if i2 != 3 {
		t.Errorf("i2 should be 3 but is %d", i1)
	}
	SetIfMissing(&s1, "s1")
	if s1 != "s1" {
		t.Errorf("s1 should be \"s1\" but is %q", s1)
	}
	SetIfMissing(&s2, "s2")
	if s2 != "3" {
		t.Errorf("s2 should be \"3\" but is %q", s2)
	}
}
