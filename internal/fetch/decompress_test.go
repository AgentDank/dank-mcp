// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"bytes"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestDecompressZstd(t *testing.T) {
	payload := []byte("hello dank-data, this is a test payload")

	var compressed bytes.Buffer
	w, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	w.Close()

	var out bytes.Buffer
	if err := decompressZstd(&compressed, &out); err != nil {
		t.Fatalf("decompressZstd: %v", err)
	}
	if !bytes.Equal(out.Bytes(), payload) {
		t.Errorf("mismatch: got %q want %q", out.String(), payload)
	}
}

func TestDecompressZstd_BadInput(t *testing.T) {
	var out bytes.Buffer
	err := decompressZstd(bytes.NewReader([]byte("not zstd")), &out)
	if err == nil {
		t.Fatal("expected error for garbage input")
	}
}
