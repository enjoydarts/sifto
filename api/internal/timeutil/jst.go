package timeutil

import "time"

var JST = time.FixedZone("JST", 9*60*60)

func NowJST() time.Time {
	return time.Now().In(JST)
}

func StartOfDayJST(t time.Time) time.Time {
	j := t.In(JST)
	return time.Date(j.Year(), j.Month(), j.Day(), 0, 0, 0, 0, JST)
}

func ParseToJST(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	var lastErr error
	for _, layout := range layouts {
		if layout == "2006-01-02 15:04:05" || layout == "2006-01-02" {
			t, err := time.ParseInLocation(layout, s, JST)
			if err == nil {
				return t.In(JST), nil
			}
			lastErr = err
			continue
		}

		t, err := time.Parse(layout, s)
		if err == nil {
			return t.In(JST), nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}
