package types

import (
	"sort"
	"time"
)

type Date struct {
	time.Time
}

type BySentOn []Postcard

var _ sort.Interface = (*BySentOn)(nil)

func (a BySentOn) Len() int      { return len(a) }
func (a BySentOn) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BySentOn) Less(i, j int) bool {
	return a[i].Meta.SentOn.Unix() > a[j].Meta.SentOn.Unix()
}
