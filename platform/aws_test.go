/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestS3PutGetEncryptedBlob(t *testing.T) {
	err := PushConfig("development")
	if err != nil {
		t.Fatal(err)
	}
	blobName := uuid.NewString()
	content := "This is a test. This is only a test."
	inStream := strings.NewReader(content)
	err = S3PutEncryptedBlob(context.Background(), blobName, inStream)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = S3DeleteBlob(context.Background(), blobName)
		if err != nil {
			t.Fatal(err)
		}
	}()
	b := bytes.Buffer{}
	err = S3GetEncryptedBlob(context.Background(), blobName, &b)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != content {
		t.Errorf("Retrieved content does not match original content: %q != %q", b.String(), content)
	}
}
