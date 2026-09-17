package codex

import (
	"github.com/klauspost/compress/zstd"
)

var zstdEncoder *zstd.Encoder

func init() {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(3)))
	if err != nil {
		return
	}
	zstdEncoder = enc
}

// compressZstd compresses data with zstd level 3. Returns nil on failure.
func compressZstd(data []byte) []byte {
	if zstdEncoder == nil {
		return nil
	}
	return zstdEncoder.EncodeAll(data, nil)
}

// isOfficialCodexURL reports whether the URL targets the official ChatGPT
// Codex backend (and thus supports zstd request encoding).
func isOfficialCodexURL(url string) bool {
	return url == codexBaseURL || 
		len(url) > len(codexBaseURL) && url[:len(codexBaseURL)] == codexBaseURL
}
