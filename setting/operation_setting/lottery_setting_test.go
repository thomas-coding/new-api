package operation_setting

import (
	"testing"
	"time"
)

func TestResolveLotteryWeeklyDayRandomWorkdayIsStableWithinWeek(t *testing.T) {
	monday := time.Date(2026, 4, 13, 9, 0, 0, 0, time.Local)
	thursday := monday.AddDate(0, 0, 3)
	nextMonday := monday.AddDate(0, 0, 7)

	first := ResolveLotteryWeeklyDay(LotteryWeeklyDayRandomWorkday, monday.Unix())
	second := ResolveLotteryWeeklyDay(LotteryWeeklyDayRandomWorkday, thursday.Unix())
	third := ResolveLotteryWeeklyDay(LotteryWeeklyDayRandomWorkday, nextMonday.Unix())

	if first < 1 || first > 5 {
		t.Fatalf("expected first resolved weekday in [1,5], got %d", first)
	}
	if second != first {
		t.Fatalf("expected stable resolved weekday within one week, got first=%d second=%d", first, second)
	}
	if third < 1 || third > 5 {
		t.Fatalf("expected next week resolved weekday in [1,5], got %d", third)
	}
}

func TestNormalizeLotteryWeeklyDayAcceptsRandomWorkday(t *testing.T) {
	if got := normalizeLotteryWeeklyDay(LotteryWeeklyDayRandomWorkday); got != LotteryWeeklyDayRandomWorkday {
		t.Fatalf("expected random workday weekly day to stay normalized, got %d", got)
	}
	if got := normalizeLotteryWeeklyDay(9); got != 0 {
		t.Fatalf("expected invalid weekly day to normalize to 0, got %d", got)
	}
}
