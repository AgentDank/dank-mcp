// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestCopyAndVerify_Match(t *testing.T) {
	payload := []byte("exactly these bytes")
	sum := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(sum[:])

	var dst bytes.Buffer
	n, err := copyAndVerify(&dst, bytes.NewReader(payload), wantHex)
	if err != nil {
		t.Fatalf("copyAndVerify: %v", err)
	}
	if n != int64(len(payload)) {
		t.Errorf("n = %d; want %d", n, len(payload))
	}
	if !bytes.Equal(dst.Bytes(), payload) {
		t.Errorf("dst bytes mismatch")
	}
}

func TestCopyAndVerify_Mismatch(t *testing.T) {
	payload := []byte("some bytes")
	wrongHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	var dst bytes.Buffer
	_, err := copyAndVerify(&dst, bytes.NewReader(payload), wrongHex)
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
}
