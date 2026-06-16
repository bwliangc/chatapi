package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// at 构造一个本地时区的时间点，weekday 由日期决定。
// 2024-01-01 是周一，2024-01-06 是周六，2024-01-07 是周日。
func at(t *testing.T, day, hour, min int) time.Time {
	t.Helper()
	return time.Date(2024, time.January, day, hour, min, 0, 0, time.Local)
}

func TestParseActiveWindow_DisabledAlwaysAllows(t *testing.T) {
	for _, w := range []*domain.MonitorActiveWindow{
		nil,
		{Enabled: false, Start: "08:00", End: "18:00"},
	} {
		rt, err := parseActiveWindow(w)
		if err != nil {
			t.Fatalf("parseActiveWindow(%v) err = %v", w, err)
		}
		if !rt.allows(at(t, 6, 3, 0)) { // 凌晨 3 点也应允许
			t.Fatalf("disabled window should allow any time")
		}
	}
}

func TestActiveWindow_SameDayRange(t *testing.T) {
	rt, err := parseActiveWindow(&domain.MonitorActiveWindow{Enabled: true, Start: "08:00", End: "18:00"})
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	cases := []struct {
		hour, min int
		want      bool
	}{
		{7, 59, false},
		{8, 0, true},  // 起点含
		{12, 0, true},
		{17, 59, true},
		{18, 0, false}, // 终点不含
		{23, 0, false},
	}
	for _, c := range cases {
		if got := rt.allows(at(t, 1, c.hour, c.min)); got != c.want {
			t.Errorf("allows(%02d:%02d) = %v, want %v", c.hour, c.min, got, c.want)
		}
	}
}

func TestActiveWindow_CrossMidnight(t *testing.T) {
	rt, err := parseActiveWindow(&domain.MonitorActiveWindow{Enabled: true, Start: "22:00", End: "06:00"})
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	cases := []struct {
		hour, min int
		want      bool
	}{
		{22, 0, true},
		{23, 30, true},
		{0, 0, true},
		{5, 59, true},
		{6, 0, false},
		{12, 0, false},
		{21, 59, false},
	}
	for _, c := range cases {
		if got := rt.allows(at(t, 1, c.hour, c.min)); got != c.want {
			t.Errorf("allows(%02d:%02d) = %v, want %v", c.hour, c.min, got, c.want)
		}
	}
}

func TestActiveWindow_Weekdays(t *testing.T) {
	// 仅工作日（周一..周五 = 1..5），时段 08:00-18:00。
	rt, err := parseActiveWindow(&domain.MonitorActiveWindow{
		Enabled:  true,
		Start:    "08:00",
		End:      "18:00",
		Weekdays: []int{1, 2, 3, 4, 5},
	})
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	if !rt.allows(at(t, 1, 9, 0)) { // 周一 09:00
		t.Errorf("Monday 09:00 should be allowed")
	}
	if rt.allows(at(t, 6, 9, 0)) { // 周六 09:00 → 星期不符
		t.Errorf("Saturday 09:00 should be blocked")
	}
	if rt.allows(at(t, 7, 9, 0)) { // 周日 09:00 → 星期不符
		t.Errorf("Sunday 09:00 should be blocked")
	}
	if rt.allows(at(t, 1, 7, 0)) { // 周一 07:00 → 星期符但时段不符
		t.Errorf("Monday 07:00 should be blocked")
	}
}

func TestActiveWindow_StartEqualsEndIsAllDay(t *testing.T) {
	// start==end 表示当天整天（仅受 weekdays 约束）。
	rt, err := parseActiveWindow(&domain.MonitorActiveWindow{
		Enabled:  true,
		Start:    "00:00",
		End:      "00:00",
		Weekdays: []int{6}, // 仅周六
	})
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	if !rt.allows(at(t, 6, 3, 0)) { // 周六凌晨 3 点
		t.Errorf("Saturday should be all-day allowed")
	}
	if rt.allows(at(t, 1, 3, 0)) { // 周一
		t.Errorf("Monday should be blocked")
	}
}

func TestParseActiveWindow_Invalid(t *testing.T) {
	cases := []*domain.MonitorActiveWindow{
		{Enabled: true, Start: "25:00", End: "18:00"},
		{Enabled: true, Start: "08:00", End: "9:99"},
		{Enabled: true, Start: "abc", End: "18:00"},
		{Enabled: true, Start: "08:00", End: "18:00", Weekdays: []int{7}},
		{Enabled: true, Start: "08:00", End: "18:00", Weekdays: []int{-1}},
	}
	for _, w := range cases {
		if _, err := parseActiveWindow(w); err == nil {
			t.Errorf("parseActiveWindow(%+v) expected error, got nil", w)
		}
	}
}
