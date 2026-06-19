package service

import (
	"testing"
	"time"
)

func TestParsePlanQuotaResetUntil(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 6, 6, 2, 30, 28, 0, location)

	tests := []struct {
		name    string
		message string
		want    time.Time
		wantOK  bool
	}{
		{
			name:    "token plan rfc3339 reset",
			message: "usage limit exceeded, 5-hour usage limit reached for Token Plan Plus (6596000/6596000 used), resets at 2026-06-06T05:00:00+08:00 (2056)",
			want:    time.Date(2026, 6, 6, 5, 0, 0, 0, location),
			wantOK:  true,
		},
		{
			name:    "weekly quota cst reset",
			message: "You have exceeded the weekly usage quota. It will reset at 2026-06-15 00:00:00 +0800 CST. We recommend upgrading your plan.",
			want:    time.Date(2026, 6, 15, 0, 0, 0, 0, location),
			wantOK:  true,
		},
		{
			name:    "duration reset after",
			message: "You have exhausted your capacity on this model. Your quota will reset after 1h40m15s.",
			want:    now.Add(time.Hour + 40*time.Minute + 15*time.Second),
			wantOK:  true,
		},
		{
			name:    "duration resets in",
			message: "Individual quota reached. Contact your administrator to enable overages. Resets in 146h54m51s.",
			want:    now.Add(146*time.Hour + 54*time.Minute + 51*time.Second),
			wantOK:  true,
		},
		{
			name:    "chinese quota reset time",
			message: "status_code=429, 您已达到每周/每月使用上限，您的限额将在 2026-06-24 01:20:15 重置。",
			want:    time.Date(2026, 6, 24, 1, 20, 15, 0, location),
			wantOK:  true,
		},
		{
			name:    "no reset time",
			message: "token plan limit exhausted",
			wantOK:  false,
		},
		{
			name:    "past reset time",
			message: "quota will reset at 2026-06-06T01:00:00+08:00",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParsePlanQuotaResetUntil(tt.message, now)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got != tt.want.Unix() {
				t.Fatalf("until = %d (%s), want %d (%s)", got, time.Unix(got, 0).In(location), tt.want.Unix(), tt.want)
			}
		})
	}
}
