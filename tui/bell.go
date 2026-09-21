package tui

import "io"

// bellByte is BEL, the byte a terminal (and anything multiplexing one: tmux,
// voro, a terminal emulator's tab badge) reads as "this program wants you".
const bellByte = '\a'

// WithBell returns m ringing the terminal bell on w whenever the TUI hands
// control back to the user: a turn ended, the run parked, or a tool approval
// or an ask is waiting. A nil w (the default) never rings.
//
// This is the only idle signal a host watching the terminal can use. nib is
// never silent while idle - the footer clock repaints every second - so "no
// output for a while" never happens, and a host that waits for it sees a
// session that finished minutes ago as still working.
//
// w must be the terminal the program renders to. The bell is written straight
// to it from Update rather than through the renderer, which has no way to emit
// a raw byte. That is safe because the renderer writes each frame in a single
// Write, so a one-byte write lands between frames and never inside an escape
// sequence.
func (m Model) WithBell(w io.Writer) Model {
	m.bell = w
	return m
}

// ringBell rings the bell unless there is no terminal to ring or ui.no_bell is
// set. Write errors are ignored: a bell that fails to sound is not worth
// interrupting the user over.
func (m Model) ringBell() {
	if m.bell == nil || m.cfg.UI.NoBell {
		return
	}
	_, _ = m.bell.Write([]byte{bellByte})
}
