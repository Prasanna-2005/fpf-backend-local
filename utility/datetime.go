package utility

import "time"

// ThisYear returns the SEC calendar dates for the current calendar year.
func ThisYear() (quarters []struct{ Start, End time.Time }, monthEnds [12]time.Time) {
	y := time.Now().Year()
	loc, _ := time.LoadLocation("America/New_York")

	// ---- quarters ----
	quarters = []struct{ Start, End time.Time }{
		{time.Date(y, 1, 1, 0, 0, 0, 0, loc), time.Date(y, 3, 31, 0, 0, 0, 0, loc)},
		{time.Date(y, 4, 1, 0, 0, 0, 0, loc), time.Date(y, 6, 30, 0, 0, 0, 0, loc)},
		{time.Date(y, 7, 1, 0, 0, 0, 0, loc), time.Date(y, 9, 30, 0, 0, 0, 0, loc)},
		{time.Date(y, 10, 1, 0, 0, 0, 0, loc), time.Date(y, 12, 31, 0, 0, 0, 0, loc)},
	}

	// ---- month ends ----
	for m := 0; m < 12; m++ {
		monthEnds[m] = time.Date(y, time.Month(m+2), 1, 0, 0, 0, 0, loc).AddDate(0, 0, -1)
	}
	return
}
