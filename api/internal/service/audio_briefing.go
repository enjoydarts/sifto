package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

const (
	AudioBriefingScheduleModeInterval     = "interval"
	AudioBriefingScheduleModeFixedSlots3x = "fixed_slots_3x"
)

func NormalizeAudioBriefingIntervalHours(hours int) int {
	switch hours {
	case 3, 6:
		return hours
	default:
		return 6
	}
}

func NormalizeAudioBriefingScheduleMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case AudioBriefingScheduleModeFixedSlots3x:
		return AudioBriefingScheduleModeFixedSlots3x
	default:
		return AudioBriefingScheduleModeInterval
	}
}

func AudioBriefingSlotKeyAt(now time.Time, intervalHours int) string {
	slot := AudioBriefingSlotStartAtForSchedule(now, AudioBriefingScheduleModeInterval, intervalHours)
	return fmt.Sprintf("%04d-%02d-%02d-%02d", slot.Year(), slot.Month(), slot.Day(), slot.Hour())
}

func AudioBriefingSlotKeyAtForSchedule(now time.Time, scheduleMode string, intervalHours int) string {
	slot := AudioBriefingSlotStartAtForSchedule(now, scheduleMode, intervalHours)
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
	return AudioBriefingSlotStartAtForSchedule(now, AudioBriefingScheduleModeInterval, intervalHours)
}

func AudioBriefingSlotStartAtForSchedule(now time.Time, scheduleMode string, intervalHours int) time.Time {
	scheduleMode = NormalizeAudioBriefingScheduleMode(scheduleMode)
	intervalHours = NormalizeAudioBriefingIntervalHours(intervalHours)
	now = now.In(timeutil.JST)
	switch scheduleMode {
	case AudioBriefingScheduleModeFixedSlots3x:
		switch {
		case now.Hour() >= 18:
			return time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, timeutil.JST)
		case now.Hour() >= 12:
			return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, timeutil.JST)
		case now.Hour() >= 6:
			return time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, timeutil.JST)
		default:
			prevDay := now.AddDate(0, 0, -1)
			return time.Date(prevDay.Year(), prevDay.Month(), prevDay.Day(), 18, 0, 0, 0, timeutil.JST)
		}
	default:
		slotHour := (now.Hour() / intervalHours) * intervalHours
		return time.Date(now.Year(), now.Month(), now.Day(), slotHour, 0, 0, 0, now.Location())
	}
}

func AudioBriefingPreviousSlotStartAtForSchedule(now time.Time, scheduleMode string, intervalHours int) time.Time {
	current := AudioBriefingSlotStartAtForSchedule(now, scheduleMode, intervalHours)
	scheduleMode = NormalizeAudioBriefingScheduleMode(scheduleMode)
	intervalHours = NormalizeAudioBriefingIntervalHours(intervalHours)
	switch scheduleMode {
	case AudioBriefingScheduleModeFixedSlots3x:
		switch current.Hour() {
		case 6:
			prev := current.AddDate(0, 0, -1)
			return time.Date(prev.Year(), prev.Month(), prev.Day(), 18, 0, 0, 0, timeutil.JST)
		case 12:
			return time.Date(current.Year(), current.Month(), current.Day(), 6, 0, 0, 0, timeutil.JST)
		default:
			return time.Date(current.Year(), current.Month(), current.Day(), 12, 0, 0, 0, timeutil.JST)
		}
	default:
		return current.Add(-time.Duration(intervalHours) * time.Hour)
	}
}

func AudioBriefingSlotLookbackHoursForSchedule(now time.Time, scheduleMode string, intervalHours int) int {
	current := AudioBriefingSlotStartAtForSchedule(now, scheduleMode, intervalHours)
	previous := AudioBriefingPreviousSlotStartAtForSchedule(now, scheduleMode, intervalHours)
	diff := current.Sub(previous)
	if diff <= 0 {
		return NormalizeAudioBriefingIntervalHours(intervalHours)
	}
	hours := int(diff / time.Hour)
	if hours <= 0 {
		return NormalizeAudioBriefingIntervalHours(intervalHours)
	}
	return hours
}
