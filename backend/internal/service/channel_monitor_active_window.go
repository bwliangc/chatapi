package service

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// 渠道监控「检测时间窗口」的解析与判断。
//
// 设计：窗口配置（domain.MonitorActiveWindow）存的是 "HH:MM" 字符串 + 星期数组，
// runner 在 Schedule 时一次性解析为下面的运行时形态（分钟 + 定长 bool 数组），
// 之后每轮 fire 用 allows(now) 做 O(1) 判断，避免每次重复解析字符串。
// 判断一律按服务器本地时区（time.Now()，非 UTC），与用户在 UI 填写的 8:00-18:00 直觉一致。

// activeWindowRuntime 是 domain.MonitorActiveWindow 的运行时解析形态。
// 零值（enabled=false）表示不限制，allows 恒为 true。
type activeWindowRuntime struct {
	enabled  bool
	startMin int     // [0, 1440)
	endMin   int     // [0, 1440)
	allDay   bool    // startMin == endMin：当天整天（仅受 weekdays 约束）
	weekdays [7]bool // 下标 = time.Weekday（0=周日..6=周六）
	anyDay   bool    // weekdays 为空：每天都允许
}

// parseActiveWindow 解析并校验窗口配置。
//   - w 为 nil 或 Enabled=false：返回不限制的运行时（allows 恒真），不报错。
//   - 字符串非法或星期越界：返回 ErrChannelMonitorInvalidActiveWindow。
func parseActiveWindow(w *domain.MonitorActiveWindow) (activeWindowRuntime, error) {
	rt := activeWindowRuntime{allDay: true, anyDay: true}
	if w == nil || !w.Enabled {
		return rt, nil
	}

	start, ok := parseHHMM(w.Start)
	if !ok {
		return activeWindowRuntime{}, ErrChannelMonitorInvalidActiveWindow
	}
	end, ok := parseHHMM(w.End)
	if !ok {
		return activeWindowRuntime{}, ErrChannelMonitorInvalidActiveWindow
	}

	rt.enabled = true
	rt.startMin = start
	rt.endMin = end
	rt.allDay = start == end

	for _, d := range w.Weekdays {
		if d < 0 || d > 6 {
			return activeWindowRuntime{}, ErrChannelMonitorInvalidActiveWindow
		}
		rt.weekdays[d] = true
	}
	rt.anyDay = len(w.Weekdays) == 0
	return rt, nil
}

// parseHHMM 解析 "HH:MM"（24 小时制）为当天分钟数 [0, 1440)。
// 第二个返回值为 false 表示格式非法。
func parseHHMM(s string) (int, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// allows 判断 now（应传入本地时区时间）是否落在检测窗口内。
func (rt activeWindowRuntime) allows(now time.Time) bool {
	if !rt.enabled {
		return true
	}
	// 星期：跨午夜窗口的后半段按"当前时刻所在星期"归属（足够直觉，且同日窗口完全准确）。
	if !rt.anyDay && !rt.weekdays[int(now.Weekday())] {
		return false
	}
	if rt.allDay {
		return true
	}
	cur := now.Hour()*60 + now.Minute()
	if rt.startMin < rt.endMin {
		return cur >= rt.startMin && cur < rt.endMin
	}
	// 跨午夜：startMin > endMin，例如 22:00-06:00 → [22:00,24:00) ∪ [00:00,06:00)
	return cur >= rt.startMin || cur < rt.endMin
}
