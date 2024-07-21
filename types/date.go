package types

import "fmt"

type Date string

func (d Date) Valid() bool {
	_, _, _, err := d.Components()
	return err == nil
}

func (d Date) Components() (year int, month int, day int, err error) {
	_, err = fmt.Sscanf(string(d), "%d-%d-%d", &year, &month, &day)
	return year, month, day, err
}
