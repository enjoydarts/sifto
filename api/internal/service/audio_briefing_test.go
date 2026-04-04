package service

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

func TestAudioBriefingSlotKeyAtThreeHourInterval(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 17, 0, 0, time.FixedZone("JST", 9*60*60))

	got := AudioBriefingSlotKeyAt(now, 3)

	if got != "2026-03-24-09" {
		t.Fatalf("AudioBriefingSlotKeyAt(..., 3) = %q, want %q", got, "2026-03-24-09")
	}
}

func TestAudioBriefingSlotKeyAtSixHourInterval(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 17, 0, 0, time.FixedZone("JST", 9*60*60))

	got := AudioBriefingSlotKeyAt(now, 6)

	if got != "2026-03-24-06" {
		t.Fatalf("AudioBriefingSlotKeyAt(..., 6) = %q, want %q", got, "2026-03-24-06")
	}
}

func TestNormalizeAudioBriefingIntervalHours(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "three stays three", in: 3, want: 3},
		{name: "six stays six", in: 6, want: 6},
		{name: "zero falls back to six", in: 0, want: 6},
		{name: "other falls back to six", in: 12, want: 6},
	}

	for _, tt := range tests {
		if got := NormalizeAudioBriefingIntervalHours(tt.in); got != tt.want {
			t.Fatalf("%s: NormalizeAudioBriefingIntervalHours(%d) = %d, want %d", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestNormalizeAudioBriefingScheduleMode(t *testing.T) {
	if got := NormalizeAudioBriefingScheduleMode("fixed_slots_3x"); got != AudioBriefingScheduleModeFixedSlots3x {
		t.Fatalf("NormalizeAudioBriefingScheduleMode(fixed_slots_3x) = %q, want %q", got, AudioBriefingScheduleModeFixedSlots3x)
	}
	if got := NormalizeAudioBriefingScheduleMode(" unknown "); got != AudioBriefingScheduleModeInterval {
		t.Fatalf("NormalizeAudioBriefingScheduleMode(unknown) = %q, want %q", got, AudioBriefingScheduleModeInterval)
	}
}

func TestAudioBriefingSlotStartAtUsesJST(t *testing.T) {
	nowUTC := time.Date(2026, 3, 24, 0, 59, 0, 0, time.UTC)
	slot := AudioBriefingSlotStartAt(nowUTC, 3)
	if slot.Location() != timeutil.JST {
		t.Fatalf("slot.Location() = %v, want JST", slot.Location())
	}
	if got := slot.Format("2006-01-02 15:04"); got != "2026-03-24 09:00" {
		t.Fatalf("slot = %s, want 2026-03-24 09:00", got)
	}
}

func TestAudioBriefingFixedSlotStartAtAndLookback(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)

	tests := []struct {
		name         string
		now          time.Time
		wantSlot     string
		wantPrev     string
		wantLookback int
	}{
		{
			name:         "morning slot looks back to previous day evening",
			now:          time.Date(2026, 3, 25, 6, 0, 0, 0, jst),
			wantSlot:     "2026-03-25 06:00",
			wantPrev:     "2026-03-24 18:00",
			wantLookback: 12,
		},
		{
			name:         "noon slot looks back six hours",
			now:          time.Date(2026, 3, 25, 12, 5, 0, 0, jst),
			wantSlot:     "2026-03-25 12:00",
			wantPrev:     "2026-03-25 06:00",
			wantLookback: 6,
		},
		{
			name:         "evening slot looks back six hours",
			now:          time.Date(2026, 3, 25, 18, 59, 0, 0, jst),
			wantSlot:     "2026-03-25 18:00",
			wantPrev:     "2026-03-25 12:00",
			wantLookback: 6,
		},
	}

	for _, tt := range tests {
		slot := AudioBriefingSlotStartAtForSchedule(tt.now, AudioBriefingScheduleModeFixedSlots3x, 6)
		if got := slot.Format("2006-01-02 15:04"); got != tt.wantSlot {
			t.Fatalf("%s: slot = %s, want %s", tt.name, got, tt.wantSlot)
		}
		prev := AudioBriefingPreviousSlotStartAtForSchedule(tt.now, AudioBriefingScheduleModeFixedSlots3x, 6)
		if got := prev.Format("2006-01-02 15:04"); got != tt.wantPrev {
			t.Fatalf("%s: prev = %s, want %s", tt.name, got, tt.wantPrev)
		}
		if got := AudioBriefingSlotLookbackHoursForSchedule(tt.now, AudioBriefingScheduleModeFixedSlots3x, 6); got != tt.wantLookback {
			t.Fatalf("%s: lookback = %d, want %d", tt.name, got, tt.wantLookback)
		}
	}
}
