package web

import "time"

// addedLabel formats when an item was added, in now's time zone.
func addedLabel(created, now time.Time) string {
	c := created.In(now.Location())
	y, m, d := c.Date()
	ny, nm, nd := now.Date()
	py, pm, pd := now.AddDate(0, 0, -1).Date()
	switch {
	case y == ny && m == nm && d == nd:
		return "Added today, " + c.Format("15:04")
	case y == py && m == pm && d == pd:
		return "Added yesterday, " + c.Format("15:04")
	case y == ny:
		return "Added " + c.Format("2 Jan")
	default:
		return "Added " + c.Format("2 Jan 2006")
	}
}
