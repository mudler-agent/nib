package tui

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mudler/nib/config"
	"github.com/mudler/nib/slash"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/types"
)

// /settings: list, show, set and reset the scalar keys of the config file from
// inside the TUI. config owns the schema (which keys exist, their types, how to
// write them without losing comments); this file owns what the running session
// does with a change.

// settingsVerb is the /settings prefix argument completion keys on.
var settingsVerb = "/" + theme.CompSettingsName + " "

// liveSettingPrefixes are the keys a change applies to without a restart,
// matched by prefix ("compaction." covers the whole block). Each has a matching
// arm in applyLiveSettings. Everything else is read once at startup (the model
// and endpoint build the client, log_level configures the logger, browser and
// agent options wire tools), so it is saved and reported as "next start".
var liveSettingPrefixes = []string{"ui.", "approval_mode", "compaction.", "tool_output_pruning."}

func isLiveSetting(key string) bool {
	for _, p := range liveSettingPrefixes {
		if key == p || strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// settingsPath is the config file /settings reads and writes: the one this
// process loaded. config.WritablePathIn applies the same first-existing-file
// search loadFromFileIn used at startup (or the embedder's single root), and
// falls back to ~/.config/nib/config.yaml when no file exists yet, which is
// also where the next start will look. It is resolved per call rather than
// stored, the same as the self-config tools resolve theirs, so /settings and
// `nib mcp add` can never disagree about which file is "the config".
func (m Model) settingsPath() string {
	return config.WritablePathIn(m.cfg.BaseDir)
}

// runSettings handles a resolved /settings action.
func (m *Model) runSettings(a slash.Action) {
	if a.SettingKey == "" {
		m.appendMessage(ChatMessage{Role: "agent", Content: fencedListing(m.settingsListing())})
		return
	}
	s, err := config.LookupSetting(a.SettingKey)
	if err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return
	}
	switch {
	case a.SettingUnset:
		m.resetSetting(s)
	case a.SettingHasValue:
		m.setSetting(s, a.SettingValue)
	default:
		m.appendMessage(ChatMessage{Role: "agent", Content: m.settingDetail(s)})
	}
}

// settingsListing renders every key with its running value and where that
// value comes from. Laid out as a fenced block, like /models, so the columns
// survive the transcript's word wrapping.
func (m Model) settingsListing() string {
	path := m.settingsPath()
	_, present, err := config.FileSettings(path)
	var b strings.Builder
	fmt.Fprintf(&b, theme.SettingsListHeader+"\n", shortenPath(path))
	if err != nil {
		fmt.Fprintf(&b, "%s\n", err)
	}
	settings := config.Settings()
	keyW := 0
	for _, s := range settings {
		keyW = max(keyW, len(s.Key))
	}
	anyPending := false
	for _, s := range settings {
		val := clipSettingValue(s.Format(m.cfg))
		if saved, ok := m.pendingSettings[s.Key]; ok {
			val, anyPending = clipSettingValue(saved)+theme.SettingsPendingMark, true
		}
		src := theme.SettingsSourceDefault
		if present[s.Key] {
			src = theme.SettingsSourceFile
		}
		fmt.Fprintf(&b, "  %-*s  %-18s %s\n", keyW, s.Key, val, src)
	}
	if anyPending {
		b.WriteString(theme.SettingsPendingNote + "\n")
	}
	b.WriteString(theme.SettingsListFooter + "\n")
	return b.String()
}

// clipSettingValue keeps a value to one short column; a custom prompt would
// otherwise take the listing over.
func clipSettingValue(v string) string {
	const w = 18
	if r := []rune(v); len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return v
}

// settingDetail renders one key: value and source, type and description,
// accepted values, and the file it lives in.
func (m Model) settingDetail(s config.Setting) string {
	path := m.settingsPath()
	_, present, _ := config.FileSettings(path)
	src := theme.SettingsSourceDefault
	if present[s.Key] {
		src = theme.SettingsSourceFile
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s = %s (%s)\n", s.Key, s.Format(m.cfg), src)
	if saved, ok := m.pendingSettings[s.Key]; ok {
		fmt.Fprintf(&b, "saved %s, applies on next start\n", saved)
	}
	b.WriteString(string(s.Type))
	if s.Doc != "" {
		b.WriteString(" · " + s.Doc)
	}
	b.WriteString("\n")
	if len(s.Values) > 0 {
		fmt.Fprintf(&b, "values: %s\n", strings.Join(s.Values, ", "))
	}
	fmt.Fprintf(&b, "file: %s", shortenPath(path))
	return b.String()
}

// setSetting validates raw, writes it, and applies whatever it changed.
func (m *Model) setSetting(s config.Setting, raw string) {
	v, err := s.Parse(raw)
	if err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return
	}
	path := m.settingsPath()
	before, _, err := config.FileSettings(path)
	if err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return
	}
	if err := config.WriteSetting(path, s.Key, v); err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return
	}
	live := m.applySettingChange(s, before, path)
	notice := fmt.Sprintf(theme.SettingsSaved, s.Key, config.FormatSettingValue(v), shortenPath(path))
	if !live {
		notice += theme.SettingsNextStart
	}
	m.appendMessage(ChatMessage{Role: "agent", Content: notice})
}

// resetSetting removes the key from the file so its default applies again.
func (m *Model) resetSetting(s config.Setting) {
	path := m.settingsPath()
	before, _, err := config.FileSettings(path)
	if err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return
	}
	removed, err := config.UnsetSetting(path, s.Key)
	if err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return
	}
	if !removed {
		m.appendMessage(ChatMessage{Role: "agent", Content: fmt.Sprintf(theme.SettingsNotSet, s.Key, shortenPath(path), s.Format(before))})
		return
	}
	live := m.applySettingChange(s, before, path)
	after, _, _ := config.FileSettings(path)
	notice := fmt.Sprintf(theme.SettingsReset, s.Key, s.Format(after), shortenPath(path))
	if !live {
		notice += theme.SettingsNextStart
	}
	m.appendMessage(ChatMessage{Role: "agent", Content: notice})
}

// applySettingChange brings the running session in line with a write to path,
// given what the file resolved to before it. It reports whether the key the
// user named took effect live.
//
// It diffs the whole file rather than applying the one value, because one
// write can move several keys: the first key written into an absent
// tool_output_pruning block seeds the block's defaults, and a reset that
// empties a block brings the block default back for its siblings. Diffing
// what the file resolves to before and after catches every such knock-on with
// no knowledge of which blocks behave that way.
//
// The named key is always carried over, even when the file already held that
// value: the running value can differ from the file's (a /yolo toggled on, a
// window the session detected), and the user asked for this one explicitly.
func (m *Model) applySettingChange(named config.Setting, before types.Config, path string) bool {
	after, _, err := config.FileSettings(path)
	if err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return false
	}
	var changed []config.Setting
	for _, s := range config.Settings() {
		if s.Key == named.Key || !reflect.DeepEqual(s.Value(before), s.Value(after)) {
			changed = append(changed, s)
		}
	}
	live := false
	for _, s := range changed {
		if !isLiveSetting(s.Key) {
			if m.pendingSettings == nil {
				m.pendingSettings = map[string]string{}
			}
			m.pendingSettings[s.Key] = s.Format(after)
			continue
		}
		s.Apply(&m.cfg, s.Value(after))
		delete(m.pendingSettings, s.Key)
		if s.Key == named.Key {
			live = true
		}
	}
	m.applyLiveSettings(changed)
	m.completion.setSettingsConfig(m.cfg)
	m.updateViewport()
	return live
}

// applyLiveSettings pushes the changed live keys from m.cfg into the running
// session. ui.* needs nothing here: the view reads m.cfg.UI on every frame.
func (m *Model) applyLiveSettings(changed []config.Setting) {
	if m.session == nil {
		return
	}
	var approval, compaction, pruning bool
	for _, s := range changed {
		switch {
		case s.Key == "approval_mode":
			approval = true
		case strings.HasPrefix(s.Key, "compaction."):
			compaction = true
		case strings.HasPrefix(s.Key, "tool_output_pruning."):
			pruning = true
		}
	}
	if approval {
		m.session.SetApprovalMode(m.cfg.ApprovalMode)
	}
	if compaction {
		m.session.SetCompaction(m.cfg.Compaction)
	}
	if pruning {
		m.session.SetToolOutputPruning(m.cfg.ToolOutputPruning)
	}
}

// settingArgItems returns the /settings argument completions for input: the
// matching keys while the key is being typed, then the key's known values
// once a space follows it. ok is false for any other input, including a
// free-form key (nothing to offer) and a value already followed by a space
// (nothing left to complete), which lets the popup close.
//
// scope names the list, so compState can reset its selection when the list
// changes from keys to values.
func settingArgItems(input string, cfg types.Config) (items []compItem, query, scope string, ok bool) {
	rest, isSettings := strings.CutPrefix(input, settingsVerb)
	if !isSettings {
		return nil, "", "", false
	}
	rest = strings.TrimLeft(rest, " \t")
	key, partial, hasValue := strings.Cut(rest, " ")
	if !hasValue {
		if strings.ContainsAny(key, "\t") {
			return nil, "", "", false
		}
		for _, s := range config.Settings() {
			desc := s.Format(cfg)
			if s.Doc != "" {
				desc += " · " + s.Doc
			}
			items = append(items, compItem{Cat: compSetting, Name: s.Key, Desc: desc, Insert: settingsVerb + s.Key + " "})
		}
		return items, key, "keys", true
	}
	partial = strings.TrimLeft(partial, " ")
	if strings.ContainsAny(partial, " \t") {
		return nil, "", "", false
	}
	s, err := config.LookupSetting(key)
	if err != nil || len(s.Values) == 0 {
		return nil, "", "", false
	}
	current := s.Format(cfg)
	for _, v := range s.Values {
		desc := ""
		if v == current {
			desc = theme.SettingsValueCurrent
		}
		items = append(items, compItem{Cat: compValue, Name: v, Desc: desc, Insert: settingsVerb + key + " " + v + " "})
	}
	items = append(items, compItem{Cat: compValue, Name: "default", Desc: theme.SettingsValueDefault, Insert: settingsVerb + key + " default "})
	return items, partial, "value:" + key, true
}
