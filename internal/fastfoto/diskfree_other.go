//go:build !unix

package fastfoto

func freeBytes(string) (uint64, bool) { return 0, false }
