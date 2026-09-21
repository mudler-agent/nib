package proc

import "time"

// KillGrace is how long a cancelled command has between SIGTERM and SIGKILL.
//
// It is shorter than the 2s WaitDelay the callers set. When a command's shell
// exits on SIGTERM but a child that ignores it keeps the output pipes open,
// Wait returns at WaitDelay; the SIGKILL must land before that, or a caller
// that exits right after Wait (nib quitting) leaves the child running.
const KillGrace = time.Second
