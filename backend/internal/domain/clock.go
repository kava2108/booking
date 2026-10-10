package domain

import (
	"fmt"
	"time"
)

// JST は診療日・診療時間の基準タイムゾーン。日本は夏時間が無いため、tzdata に依存しない固定オフセットを使う。
// 単一医療機関・国内運用が前提（多拠点・海外にするなら設定化する）。
var JST = time.FixedZone("JST", 9*60*60)

// StartOfDay は t を JST に直した日の 00:00 を返す。
func StartOfDay(t time.Time) time.Time {
	j := t.In(JST)
	return time.Date(j.Year(), j.Month(), j.Day(), 0, 0, 0, 0, JST)
}

// ParseDate は "2006-01-02" 形式を JST の 00:00 に変換する。存在しない日付はエラー。
func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, JST)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
	}
	return t, nil
}

// ParseClock は "HH:MM"（24 時間表記・ゼロ埋め）を 0:00 からの経過時間に変換する。
func ParseClock(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != 5 {
		return 0, fmt.Errorf("invalid time %q (want HH:MM)", s)
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

// FormatClock は 0:00 からの経過時間を "HH:MM" にする。
func FormatClock(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Hours()), int(d.Minutes())%60)
}
