package theme

// /settings copy (tui/settings.go). Kept apart from copy.go only so the
// command's strings sit together; the voice is the same: calm, lowercase, no
// emoji.
const (
	// Built-in `/` completion entry, as in copy.go's Comp* block.
	CompSettingsName = "settings"
	CompSettingsDesc = "view or change a config setting"

	// SettingsListHeader heads the /settings listing; %s is the config file
	// /settings writes to.
	SettingsListHeader = "settings · %s"
	// SettingsListFooter closes the listing with how to change a value.
	SettingsListFooter = "/settings <key> <value> to change · /settings <key> default to reset"
	// SettingsPendingNote explains the listing's marker on a saved value the
	// running session has not picked up.
	SettingsPendingNote = "* saved, applies on next start"
	SettingsPendingMark = " *"

	SettingsSourceFile    = "file"
	SettingsSourceDefault = "default"

	// SettingsSaved confirms a write: key, value, file.
	SettingsSaved = "%s = %s · saved to %s"
	// SettingsNextStart is appended to SettingsSaved for a key the running
	// session cannot change under itself.
	SettingsNextStart = " · applies on next start"
	// SettingsReset confirms an unset: key, the default now in effect, file.
	SettingsReset = "%s reset to default %s · removed from %s"
	// SettingsNotSet answers an unset of a key the file never had.
	SettingsNotSet = "%s is not set in %s · default %s already applies"

	// Completion descriptions for the value popup.
	SettingsValueCurrent = "current"
	SettingsValueDefault = "remove from the file, use the default"
)
