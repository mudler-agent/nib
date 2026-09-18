//go:build ignore

// gen_catalog fetches the oh-my-pi models.json from the upstream
// repository and writes it as zstd-compressed data to catalog.json.zst.
//
// Usage:
//
//	go generate ./llmprovider/catalog/...
//
// The source URL and commit are pinned here so the catalog version is
// reproducible. To update, change the commit below and re-run.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/klauspost/compress/zstd"
)

// sourceURL is the raw GitHub URL of oh-my-pi's models.json at a pinned
// commit. Update the commit hash to refresh the catalog.
const sourceURL = "https://raw.githubusercontent.com/can1357/oh-my-pi/78b753124d11f8dd3ae73e2524125890ff7c977e/packages/catalog/src/models.json"

func main() {
	resp, err := http.Get(sourceURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "fetch: %s\n", resp.Status)
		os.Exit(1)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(19)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "zstd init: %v\n", err)
		os.Exit(1)
	}
	compressed := enc.EncodeAll(raw, nil)
	enc.Close()

	if err := os.WriteFile("catalog.json.zst", compressed, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("catalog.json.zst: %d bytes (from %d raw)\n", len(compressed), len(raw))
}
