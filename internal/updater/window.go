package updater

import "time"

// IsWithinWindow reports whether t (in its location) falls in the half-open
// interval [startHour, endHour). Hours are 0-23. Assumes endHour > startHour
// (no midnight crossing).
func IsWithinWindow(t time.Time, startHour, endHour int) bool {
	h := t.Hour()
	return h >= startHour && h < endHour
}
