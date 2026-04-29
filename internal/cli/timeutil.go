package cli

import "time"

func todayFromTime() string {
	return time.Now().Format("2006-01-02")
}
