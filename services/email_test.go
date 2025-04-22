/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package services

import (
	"github.com/google/uuid"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"testing"
)

func TestSendAuthenticationCodeViaEmail(t *testing.T) {
	if err := platform.PushConfig("staging"); err != nil {
		t.Fatal(err)
	}
	defer platform.PopConfig()
	if err := SendAuthenticationCodeViaEmail("dan@whisper-project.org", uuid.NewString()); err != nil {
		t.Fatal(err)
	}
}
