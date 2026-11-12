//go:build unix

package claudecode

import (
	"os/exec"
	"syscall"
)

// Its own process group keeps the terminal's Ctrl-C from reaching claude: the caller decides
// when to cancel, through the context.
func detachFromTerminal(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
