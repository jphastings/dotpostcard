//go:build unix

package fastfoto

import "golang.org/x/sys/unix"

// freeBytes is how much space is left on dir's filesystem, if that can be told.
func freeBytes(dir string) (uint64, bool) {
	var st unix.Statfs_t
	if unix.Statfs(dir, &st) != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true
}
