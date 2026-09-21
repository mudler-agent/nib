package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// To refresh the embedded catalog, run:
//
//	go generate ./llmprovider/catalog/...
//
// This fetches the upstream oh-my-pi models.json, compresses it with
// zstd, and writes catalog.json.zst. Commit the result.
//
//go:generate go run gen_catalog.go
//go:embed catalog.json.zst
var compressedCatalog []byte

var (
	loadOnce  sync.Once
	loadErr   error
	catalogDB map[string]map[string]Model // provider -> modelID -> Model

	// Secondary indices built on load for cross-provider lookup.
	byModelID     map[string][]Model // modelID -> matching models
	byBaseURLHost map[string][]Model // host -> matching models
	modelIDs      []string           // byModelID's keys, sorted
)

// Load decompresses and parses the embedded catalog. It is called lazily by
// Lookup and is safe to call from multiple goroutines.
func Load() error {
	loadOnce.Do(func() {
		raw, err := decompress(compressedCatalog)
		if err != nil {
			loadErr = fmt.Errorf("catalog: decompress: %w", err)
			return
		}
		if err := json.Unmarshal(raw, &catalogDB); err != nil {
			loadErr = fmt.Errorf("catalog: parse: %w", err)
			return
		}
		buildIndices()
	})
	return loadErr
}

func decompress(data []byte) ([]byte, error) {
	r, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.DecodeAll(data, nil)
}

// buildIndices walks providers and their models in sorted order. The same
// model ID is often served by several providers with different limits, and
// Lookup returns the first acceptable entry of a bucket, so the bucket order
// decides the answer. Built from map iteration, that order changed from run
// to run, and so did the max_tokens nib sent for a model such as glm-5.2.
func buildIndices() {
	byModelID = make(map[string][]Model)
	byBaseURLHost = make(map[string][]Model)
	for _, provider := range sortedKeys(catalogDB) {
		models := catalogDB[provider]
		for _, id := range sortedKeys(models) {
			m := models[id]
			byModelID[m.ID] = append(byModelID[m.ID], m)
			if h := hostOf(m.BaseURL); h != "" {
				byBaseURLHost[h] = append(byBaseURLHost[h], m)
			}
		}
	}
	modelIDs = sortedKeys(byModelID)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Lookup finds a model in the catalog using progressively looser matching:
//  1. Exact provider + model ID
//  2. Base URL host + exact model ID (provider-specific via endpoint)
//  3. Model ID across all providers (prefers non-openrouter entries)
//  4. Model ID suffix match (strip "org/" prefix from catalog IDs)
//
// Returns nil, false when nothing matches.
func Lookup(providerName, modelID, baseURL string) (*Model, bool) {
	if err := Load(); err != nil {
		return nil, false
	}
	providerName = strings.ToLower(providerName)
	modelID = strings.ToLower(modelID)

	// 1. Exact provider + model ID.
	if models, ok := catalogDB[providerName]; ok {
		if m, ok := models[modelID]; ok {
			return &m, true
		}
	}

	// 2. Base URL host + exact model ID — more precise than a bare
	//    model-ID search because it ties the match to the actual endpoint.
	host := hostOf(baseURL)
	if host != "" {
		if matches, ok := byBaseURLHost[host]; ok {
			for i := range matches {
				if strings.EqualFold(matches[i].ID, modelID) {
					return &matches[i], true
				}
			}
		}
	}

	// 3. Model ID across all providers. When the same model ID is served
	//    by multiple providers, prefer a non-openrouter entry (openrouter
	//    has the omit flag, which would suppress max_tokens even for a
	//    direct provider that needs it).
	if matches, ok := byModelID[modelID]; ok && len(matches) > 0 {
		best := 0
		for i := range matches {
			if !matches[i].Compat.IsOpenRouterHost {
				best = i
				break
			}
		}
		return &matches[best], true
	}

	// 4. Suffix match: strip "org/" prefix from catalog IDs.
	//    e.g. user model "glm-5.2" matches catalog ID "zai-org/glm-5.2".
	for _, id := range modelIDs {
		matches := byModelID[id]
		for i := range matches {
			if suffixMatch(matches[i].ID, modelID) {
				return &matches[i], true
			}
		}
	}

	return nil, false
}

// suffixMatch reports whether catalogID ends with modelID after stripping an
// optional "org/" prefix from catalogID. e.g. "zai-org/glm-5.2" matches "glm-5.2".
func suffixMatch(catalogID, modelID string) bool {
	catalogID = strings.ToLower(catalogID)
	if idx := strings.LastIndex(catalogID, "/"); idx >= 0 {
		catalogID = catalogID[idx+1:]
	}
	return catalogID == modelID
}

func hostOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

// Stats returns the number of providers and models in the catalog.
func Stats() (providers, models int) {
	if err := Load(); err != nil {
		return 0, 0
	}
	providers = len(catalogDB)
	for _, m := range catalogDB {
		models += len(m)
	}
	return
}
