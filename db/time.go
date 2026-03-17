package db

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const TimeLayout = "2006-01-02 15:04:05"

var CronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// FormatLocal converts a UTC timestamp string to the local timezone.
func FormatLocal(utcStr string) string {
	t, err := time.Parse(TimeLayout, utcStr)
	if err != nil {
		return utcStr
	}
	return t.Local().Format(TimeLayout)
}

func matchesDateLocal(utcStr, date string) bool {
	return strings.HasPrefix(FormatLocal(utcStr), date)
}
