package domain

// MonitorActiveWindow 渠道监控的检测时间窗口（按服务器本地时区判断）。
//
// Enabled=false（零值，也是历史数据默认）表示 7×24 全天检测，与未配置窗口时行为完全一致。
// 仅当 Enabled=true 时 Start/End/Weekdays 才参与判断。
type MonitorActiveWindow struct {
	Enabled bool `json:"enabled"`

	// Start/End 为本地时间 "HH:MM"（24 小时制，零填充，如 "08:00"）。
	//   - Start == End：当天整天生效（仅受 Weekdays 约束，等价于不限时段）。
	//   - Start <  End：同日区间 [Start, End)，例如 "08:00"-"18:00"。
	//   - Start >  End：跨午夜区间，例如 "22:00"-"06:00"。
	Start string `json:"start"`
	End   string `json:"end"`

	// Weekdays 允许检测的星期，取值 0=周日 .. 6=周六（与 Go time.Weekday、JS Date.getDay 一致）。
	// 空表示每天都允许。跨午夜窗口的后半段按"当前时刻所在星期"判断。
	Weekdays []int `json:"weekdays,omitempty"`
}
