package checker

import (
	"strconv"
	"strings"
	"time"

	"lumipluse-backend/internal/pkg/utils"
)

// 告警静默相关设置项（可在「通知管理」页配置，读取走内存缓存，改完立即生效）。
const (
	// settingCooldownMinutes 同一服务的告警冷却窗口（分钟）。空值/0 表示不限制。
	settingCooldownMinutes = "alert_cooldown_minutes"
	// settingQuietStart / settingQuietEnd 免打扰时段（HH:mm，北京时间）。
	// 两者都填写才生效；start > end 表示跨零点（例如 23:00-07:00）。
	settingQuietStart = "quiet_hours_start"
	settingQuietEnd   = "quiet_hours_end"
)

// maxCooldownMinutes 冷却窗口上限（24 小时），避免误填一个大数字后彻底收不到告警。
const maxCooldownMinutes = 24 * 60

// alertPolicy 一次告警判定所用的策略快照
type alertPolicy struct {
	cooldown time.Duration
	// quietStart / quietEnd 为北京时间当天的分钟数，quietOn 表示免打扰是否启用
	quietStart int
	quietEnd   int
	quietOn    bool
}

// loadAlertPolicy 从设置里读取告警策略
func loadAlertPolicy() alertPolicy {
	var p alertPolicy

	if minutes, err := strconv.Atoi(strings.TrimSpace(utils.GetSetting(settingCooldownMinutes))); err == nil && minutes > 0 {
		if minutes > maxCooldownMinutes {
			minutes = maxCooldownMinutes
		}
		p.cooldown = time.Duration(minutes) * time.Minute
	}

	start, okStart := parseClock(utils.GetSetting(settingQuietStart))
	end, okEnd := parseClock(utils.GetSetting(settingQuietEnd))
	// 起止相同视为未启用（否则会变成全天静音）
	if okStart && okEnd && start != end {
		p.quietStart = start
		p.quietEnd = end
		p.quietOn = true
	}

	return p
}

// parseClock 解析 HH:mm，返回当天的分钟数（0~1439）
func parseClock(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	hour, errH := strconv.Atoi(strings.TrimSpace(parts[0]))
	minute, errM := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

// quietNow 判断给定时刻是否落在免打扰时段内（按北京时间）。
// start < end 为同日区间，start > end 表示跨零点。
func (p alertPolicy) quietNow(now time.Time) bool {
	if !p.quietOn {
		return false
	}
	cst := now.In(time.FixedZone("CST", 8*3600))
	cur := cst.Hour()*60 + cst.Minute()
	if p.quietStart < p.quietEnd {
		return cur >= p.quietStart && cur < p.quietEnd
	}
	return cur >= p.quietStart || cur < p.quietEnd
}

// allowAlert 判断服务这次异常告警是否还在冷却窗口内。
// 允许发送时会记录本次发送时间（状态随服务删除一同清理）。
func (hc *HealthChecker) allowAlert(serviceID int64) bool {
	cooldown := loadAlertPolicy().cooldown
	if cooldown <= 0 {
		return true
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	st, ok := hc.states[serviceID]
	if !ok {
		// 正常情况下状态一定存在（探测时创建），缺失时不做限制
		return true
	}
	if !st.lastAlertAt.IsZero() && time.Since(st.lastAlertAt) < cooldown {
		return false
	}
	st.lastAlertAt = time.Now()
	return true
}
