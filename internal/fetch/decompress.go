// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// decompressZstd streams a zstd-compressed reader into dst.
func decompressZstd(src io.Reader, dst io.Writer) error {
	dec, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("zstd reader: %w", err)
	}
	defer dec.Close()
	if _, err := io.Copy(dst, dec); err != nil {
		return fmt.Errorf("zstd decode: %w", err)
	}
	return nil
}
