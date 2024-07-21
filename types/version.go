package types

import "fmt"

type Version struct {
	Major uint8
	Minor uint8
	Patch uint8
}

func MustParseVersion(str string) Version {
	var ver Version
	if _, err := fmt.Sscanf(str, "%d.%d.%d", &ver.Major, &ver.Minor, &ver.Patch); err != nil {
		panic(fmt.Errorf("unable to parse version string: %s", str))
	}

	if ver.Major >= 128 || ver.Minor >= 128 || ver.Patch >= 128 {
		panic(fmt.Errorf("version number is too large for existing file structure"))
	}

	return ver
}

func (v Version) LessThan(o Version) bool {
	return o.Major > v.Major && o.Minor > v.Minor && o.Patch > v.Patch
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
