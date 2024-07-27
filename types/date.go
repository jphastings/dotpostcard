package types

import (
	"fmt"
	"sort"
)

type Date string

func (d Date) Valid() bool {
	_, _, _, err := d.Components()
	return err == nil
}

func (d Date) Components() (year int, month int, day int, err error) {
	_, err = fmt.Sscanf(string(d), "%d-%d-%d", &year, &month, &day)
	return year, month, day, err
}

func (d Date) sortable() int {
	year, mon, day, err := d.Components()
	if err != nil {
		return 0
	}

	return year*10000 + mon*100 + day
}

type BySentOn []Postcard

var _ sort.Interface = (*BySentOn)(nil)

func (a BySentOn) Len() int      { return len(a) }
func (a BySentOn) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BySentOn) Less(i, j int) bool {
	return a[i].Meta.SentOn.sortable() > a[j].Meta.SentOn.sortable()
}
