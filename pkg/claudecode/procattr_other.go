//go:build !unix

package claudecode

import "os/exec"

func detachFromTerminal(*exec.Cmd) {}
