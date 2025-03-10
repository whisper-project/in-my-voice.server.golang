/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"testing"
	"time"

	"github.com/whisper-project/in-my-voice.server.golang/platform"

	"github.com/google/uuid"
)

func TestLaunchDataInterface(t *testing.T) {
	clientId := uuid.NewString()
	l := &LaunchData{ClientId: clientId}
	var n *LaunchData
	platform.StorableInterfaceTester(t, l, "launch-data:", clientId)
	platform.StructPointerInterfaceTester(t, n, l, *l, "launch-data:", clientId)
}

func TestNewLaunchData(t *testing.T) {
	clientId := uuid.NewString()
	profileId := uuid.NewString()
	now := time.Now().UnixMilli()
	l := NewLaunchData("test", clientId, profileId)
	if l.ClientType != "test" {
		t.Errorf("NewLaunchData returned wrong client type. Got %s, Want %s", l.ClientType, "device")
	}
	if l.ClientId != clientId {
		t.Errorf("NewLaunchData returned wrong client id. Got %s, Want %s", l.ClientId, clientId)
	}
	if l.ProfileId != profileId {
		t.Errorf("NewLaunchData returned wrong profile id. Got %s, Want %s", l.ProfileId, profileId)
	}
	if now > l.Start {
		t.Errorf("NewLaunchData returned an early start. Got %v, Want no later than %v", l.Start, now)
	}
	if l.End != 0 {
		t.Errorf("NewLaunchData returned the wrong end. Got %v, want 0", l.End)
	}
}

func TestClientWhisperConversationsInterface(t *testing.T) {
	clientId := uuid.NewString()
	platform.StorableInterfaceTester(t, ClientWhisperConversations(clientId), "client-whisper-conversations:", clientId)
}
