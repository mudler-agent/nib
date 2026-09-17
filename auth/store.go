package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// storeFile is the on-disk JSON layout. Version lets us migrate the format
// later without guessing.
type storeFile struct {
	Version     int                   `json:"version"`
	Credentials map[string]Credential `json:"credentials"` // providerID → credential
}

const storeVersion = 1

// Store persists credentials as a single JSON file at Path. The file is
// written atomically (temp + rename, same directory), with 0o600 file mode
// under a 0o700 directory — the same discipline as chat/sessionstore.go.
// Concurrent writers are not serialized by the store; nib is single-process.
type Store struct {
	Path string
}

// NewStore returns a store backed by path. The file is not created until the
// first Save; Load and Delete tolerate a missing file.
func NewStore(path string) *Store {
	return &Store{Path: path}
}

func (s *Store) dir() string { return filepath.Dir(s.Path) }

// load reads and parses the store file. A missing file returns an empty
// storeFile (not an error) so callers can treat "nothing stored yet" and
// "file exists" uniformly.
func (s *Store) load() (storeFile, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return storeFile{Version: storeVersion, Credentials: map[string]Credential{}}, nil
		}
		return storeFile{}, fmt.Errorf("auth: read %s: %w", s.Path, err)
	}
	var sf storeFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return storeFile{}, fmt.Errorf("auth: parse %s: %w", s.Path, err)
	}
	if sf.Credentials == nil {
		sf.Credentials = map[string]Credential{}
	}
	return sf, nil
}

// save writes sf atomically, creating the directory with 0o700 if needed.
func (s *Store) save(sf storeFile) error {
	if sf.Version == 0 {
		sf.Version = storeVersion
	}
	dir := s.dir()
	// 0o700: the credentials file holds API keys and OAuth tokens. No other
	// local user should be able to read, list, or traverse this directory.
	// Matches the precedent in chat/sessionstore.go and setup/write.go.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("auth: create %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("auth: chmod %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("auth: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("auth: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// os.CreateTemp creates with 0o600 by default on Unix; enforce it so a
	// umask of 0 does not leave the temp file world-readable before rename.
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("auth: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("auth: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("auth: close: %w", err)
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("auth: rename: %w", err)
	}
	return nil
}

// Save stores cred for cred.ProviderID, replacing any existing credential for
// that provider (single-account model). Sets AuthorizedAt to now for OAuth
// credentials that do not already have it.
func (s *Store) Save(cred Credential) error {
	if cred.ProviderID == "" {
		return fmt.Errorf("auth: Save: empty provider_id")
	}
	if cred.Kind == CredentialOAuth && cred.AuthorizedAt.IsZero() {
		cred.AuthorizedAt = time.Now()
	}
	sf, err := s.load()
	if err != nil {
		return err
	}
	sf.Credentials[cred.ProviderID] = cred
	return s.save(sf)
}

// Get returns the stored credential for providerID, or ok=false if none.
func (s *Store) Get(providerID string) (Credential, bool, error) {
	sf, err := s.load()
	if err != nil {
		return Credential{}, false, err
	}
	cred, ok := sf.Credentials[providerID]
	return cred, ok, nil
}

// Delete removes the stored credential for providerID. A missing entry is not
// an error — the caller (logout) wants it gone, and it already is.
func (s *Store) Delete(providerID string) error {
	sf, err := s.load()
	if err != nil {
		return err
	}
	delete(sf.Credentials, providerID)
	return s.save(sf)
}

// All returns every stored credential, sorted by providerID for stable output.
// Used by `nib login --list` and the /logout picker.
func (s *Store) All() ([]Credential, error) {
	sf, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Credential, 0, len(sf.Credentials))
	for _, c := range sf.Credentials {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProviderID < out[j].ProviderID })
	return out, nil
}
