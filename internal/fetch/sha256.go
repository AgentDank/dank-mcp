// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// copyAndVerify streams src into dst while computing a sha256. After EOF,
// the computed digest is compared against wantHex. Returns the number of
// bytes copied, or an error if the digest does not match.
func copyAndVerify(dst io.Writer, src io.Reader, wantHex string) (int64, error) {
	h := sha256.New()
	tee := io.TeeReader(src, h)
	n, err := io.Copy(dst, tee)
	if err != nil {
		return n, fmt.Errorf("copy: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != wantHex {
		return n, fmt.Errorf("sha256 mismatch: expected %s, got %s", wantHex, got)
	}
	return n, nil
}
