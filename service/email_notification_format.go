package service

import (
	"fmt"
	"strings"
	"time"
)

func formatEmailTimestamp(timestamp int64) string {
	if timestamp <= 0 {
		return "-"
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05 MST")
}

func formatEmailDuration(duration time.Duration, locale string) string {
	if duration <= 0 {
		switch NormalizeEmailTemplateLocale(locale) {
		case "zh-CN":
			return "即将恢复"
		case "zh-TW":
			return "即將恢復"
		default:
			return "restoring now"
		}
	}
	totalMinutes := int64((duration + time.Minute - 1) / time.Minute)
	days := totalMinutes / (24 * 60)
	hours := totalMinutes % (24 * 60) / 60
	minutes := totalMinutes % 60
	parts := make([]string, 0, 3)
	switch NormalizeEmailTemplateLocale(locale) {
	case "zh-CN":
		if days > 0 {
			parts = append(parts, fmt.Sprintf("%d 天", days))
		}
		if hours > 0 {
			parts = append(parts, fmt.Sprintf("%d 小时", hours))
		}
		if minutes > 0 || len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("%d 分钟", minutes))
		}
	case "zh-TW":
		if days > 0 {
			parts = append(parts, fmt.Sprintf("%d 天", days))
		}
		if hours > 0 {
			parts = append(parts, fmt.Sprintf("%d 小時", hours))
		}
		if minutes > 0 || len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("%d 分鐘", minutes))
		}
	default:
		if days > 0 {
			parts = append(parts, pluralizeDuration(days, "day"))
		}
		if hours > 0 {
			parts = append(parts, pluralizeDuration(hours, "hour"))
		}
		if minutes > 0 || len(parts) == 0 {
			parts = append(parts, pluralizeDuration(minutes, "minute"))
		}
	}
	return strings.Join(parts, " ")
}

func pluralizeDuration(value int64, unit string) string {
	if value == 1 {
		return fmt.Sprintf("%d %s", value, unit)
	}
	return fmt.Sprintf("%d %ss", value, unit)
}
