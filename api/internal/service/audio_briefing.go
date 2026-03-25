package service

import (
	"fmt"
	"time"

	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

func NormalizeAudioBriefingIntervalHours(hours int) int {
	switch hours {
	case 3, 6:
		return hours
	default:
		return 6
	}
}

func AudioBriefingSlotKeyAt(now time.Time, intervalHours int) string {
	slot := AudioBriefingSlotStartAt(now, intervalHours)
	return fmt.Sprintf("%04d-%02d-%02d-%02d", slot.Year(), slot.Month(), slot.Day(), slot.Hour())
}

func AudioBriefingManualSlotKeyAt(now time.Time) string {
	now = now.In(timeutil.JST)
	return fmt.Sprintf(
		"manual-%04d-%02d-%02d-%02d%02d%02d-%09d",
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		now.Minute(),
		now.Second(),
		now.Nanosecond(),
	)
}

func AudioBriefingSlotStartAt(now time.Time, intervalHours int) time.Time {
	intervalHours = NormalizeAudioBriefingIntervalHours(intervalHours)
	now = now.In(timeutil.JST)
	slotHour := (now.Hour() / intervalHours) * intervalHours
	return time.Date(now.Year(), now.Month(), now.Day(), slotHour, 0, 0, 0, now.Location())
}
