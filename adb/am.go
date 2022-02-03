package adb

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func (adb *ADB) AmStart(pkg, activity string) error {
	return adb.Run(fmt.Sprintf("am start -n %s/%s.%s >/dev/null 2>&1", pkg, pkg, activity), nil, nil)
}

func (adb *ADB) AmKill(pkg string) error {
	return adb.Run(fmt.Sprintf("am force-stop %s >/dev/null 2>&1", pkg), nil, nil)
}

func (adb *ADB) AmEnsure(pkg, activity string) error {
	running, err := adb.TopActivity()
	if err != nil || running == pkg {
		return err
	}
	return adb.AmStart(pkg, activity)
}

var activityRE *regexp.Regexp

func getActivityRE() *regexp.Regexp {
	if activityRE != nil {
		return activityRE
	}
	activityRE = regexp.MustCompile(`\d+:([^/]+)/[^\s]+\s+\(top-activity\)`)
	return activityRE
}

func (adb *ADB) TopActivity() (pkg string, err error) {
	buf := bytes.NewBuffer(nil)
	err = adb.Run("dumpsys activity 2>&1 | grep 'top-activity' | head -n 1", buf, nil)
	if err != nil {
		return
	}
	d := strings.TrimSpace(buf.String())
	res := getActivityRE().FindStringSubmatch(d)
	if len(res) == 2 {
		pkg = res[1]
	}

	return
}
