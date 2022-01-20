package adb

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

func (adb *ADB) Setting(namespace, property string) (string, error) {
	buf := bytes.NewBuffer(nil)
	err := adb.Run(
		fmt.Sprintf("settings get '%s' '%s'", namespace, property),
		buf,
		nil,
	)
	return strings.TrimSpace(buf.String()), err
}

func (adb *ADB) SetSetting(namespace, property, value string) error {
	return adb.Run(
		fmt.Sprintf("settings put '%s' '%s' '%s'", namespace, property, value),
		nil,
		nil,
	)
}

func (adb *ADB) Brightness() (int, error) {
	v, err := adb.Setting("system", "screen_brightness")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(v))
}

func (adb *ADB) SetBrightness(n int) error {
	return adb.SetSetting("system", "screen_brightness", strconv.Itoa(n))
}

func (adb *ADB) ShowTouches() (bool, error) {
	v, err := adb.Setting("system", "show_touches")
	return v == "1", err
}

func (adb *ADB) SetShowTouches(n bool) error {
	v := "0"
	if n {
		v = "1"
	}
	return adb.SetSetting("system", "show_touches", v)
}
