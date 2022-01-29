package adb

import (
	"fmt"
	"strings"
	"time"
)

func (adb *ADB) Tap(x, y int) error {
	return adb.Run(fmt.Sprintf("input tap %d %d >/dev/null 2>&1", x, y), nil, nil)
}

func (adb *ADB) TapQuick(x, y int) error {
	return adb.Run(fmt.Sprintf("input tap %d %d >/dev/null 2>&1 &", x, y), nil, nil)
}

func (adb *ADB) Drag(x0, y0, x1, y1 int, dur time.Duration) error {
	return adb.Run(
		fmt.Sprintf(
			"input swipe %d %d %d %d %d >/dev/null 2>&1",
			x0,
			y0,
			x1,
			y1,
			dur.Milliseconds(),
		),
		nil,
		nil,
	)
}

func (adb *ADB) Hold(x, y int, dur time.Duration) error {
	return adb.Drag(x, y, x, y, dur)
}

func (adb *ADB) Text(s string) error {
	s = strings.ReplaceAll(s, "'", "'\"'\"'")
	return adb.Run(fmt.Sprintf("input text '%s' > /dev/null 2>&1", s), nil, nil)
}
