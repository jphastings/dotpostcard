//go:build !linux && !windows && !darwin

package webp

const libraryName = ""

func dlopen(name string) (uintptr, error) {
	return 0, fmt.Errorf("platform unsupported")
}
