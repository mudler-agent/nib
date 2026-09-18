package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
)

//go:embed catalog.json.zst
var compressedCatalog []byte

var (
	loadOnce  sync.Once
	loadErr   error
	catalogDB map[string]map[string]Model // provider -> modelID -> Model

	// Secondary indices built on load for cross-provider lookup.
	byModelID     map[string][]Model          // modelID -> matching models
	byBaseURLHost map[string][]Model          // host -> matching models
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

func buildIndices() {
	byModelID = make(map[string][]Model)
	byBaseURLHost = make(map[string][]Model)
	for _, models := range catalogDB {
		for _, m := range models {
			byModelID[m.ID] = append(byModelID[m.ID], m)
			if h := hostOf(m.BaseURL); h != "" {
				byBaseURLHost[h] = append(byBaseURLHost[h], m)
			}
		}
	}
}

// Lookup finds a model in the catalog using progressively looser matching:
//  1. Exact provider + model ID
//  2. Model ID across all providers
//  3. Base URL host + model ID
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

	// 2. Model ID across all providers (first match wins).
	if matches, ok := byModelID[modelID]; ok && len(matches) > 0 {
		return &matches[0], true
	}

	// 3. Base URL host + model ID.
	if h := hostOf(baseURL); h != "" {
		if matches, ok := byBaseURLHost[h]; ok {
			for i := range matches {
				if strings.EqualFold(matches[i].ID, modelID) {
					return &matches[i], true
				}
			}
		}
	}

	// 4. Suffix match: strip "org/" prefix from catalog IDs.
	//    e.g. user model "glm-5.2" matches catalog ID "zai-org/glm-5.2".
	for _, matches := range byModelID {
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
