// Package inprogress marks a file as still being written, in the way Finder understands.
// It is cosmetic: every failure is ignored, so a volume without support never breaks a build.
package inprogress

import (
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const fractionAttr = "com.apple.progress.fractionCompleted"

// Finder greys a file out, with a progress bar, when it has this creation date (the day the
// Macintosh was launched) and a fraction attribute. Safari's downloads use the same pair.
var marker = time.Date(1984, time.January, 24, 8, 0, 0, 0, time.UTC)

// Mark shows the file at path as in progress, at zero.
func Mark(path string) {
	setCreationDate(path, marker)
	SetFraction(path, 0)
}

// SetFraction shows the file at path as that much complete, from 0 to 1.
func SetFraction(path string, fraction float64) {
	fraction = min(max(fraction, 0), 1)
	_ = unix.Setxattr(path, fractionAttr, []byte(strconv.FormatFloat(fraction, 'f', -1, 64)), 0)
}

// Clear shows the file at path as finished.
func Clear(path string) {
	_ = unix.Removexattr(path, fractionAttr)
	setCreationDate(path, time.Now())
}

func setCreationDate(path string, t time.Time) {
	ts := unix.NsecToTimespec(t.UnixNano())
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&ts)), unsafe.Sizeof(ts))
	attrs := &unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_CRTIME}
	_ = unix.Setattrlist(path, attrs, buf, 0)
}
