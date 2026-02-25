package investlog

import "time"

const shanghaiTimeZoneName = "Asia/Shanghai"

var shanghaiLocation = loadShanghaiLocation()

func loadShanghaiLocation() *time.Location {
	location, err := time.LoadLocation(shanghaiTimeZoneName)
	if err != nil {
		return time.FixedZone(shanghaiTimeZoneName, 8*60*60)
	}
	return location
}

// NowInShanghai returns current time in Asia/Shanghai.
func NowInShanghai() time.Time {
	return time.Now().In(shanghaiLocation)
}

// TodayISOInShanghai returns current date using YYYY-MM-DD in Asia/Shanghai.
func TodayISOInShanghai() string {
	return NowInShanghai().Format("2006-01-02")
}

// NowRFC3339InShanghai returns current RFC3339 timestamp in Asia/Shanghai.
func NowRFC3339InShanghai() string {
	return NowInShanghai().Format(time.RFC3339)
}
