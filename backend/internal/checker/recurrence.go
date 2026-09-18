package checker

import (
	"time"

	"lumipluse-backend/internal/model"
)

// 维护计划的周期重复。
//
// 设计：同一个维护计划只保留「当前这次窗口 + 下一次窗口」。
// 窗口结束后由 reconcileMaintenanceStatus 调用 nextMaintenanceWindow 把
// scheduled_start / scheduled_end 推进到下一个窗口，状态回到 scheduled；
// 若已超过 recurrence_until，则保持 completed 不再重复。
const (
	recurrenceDaily   = "daily"
	recurrenceWeekly  = "weekly"
	recurrenceMonthly = "monthly"
)

// normalizeRecurrence 校正周期参数，返回 false 表示应按一次性窗口处理
func normalizeRecurrence(m *model.Maintenance) bool {
	switch m.Recurrence {
	case recurrenceDaily, recurrenceWeekly, recurrenceMonthly:
	default:
		m.Recurrence = ""
		return false
	}
	if m.RecurrenceInterval < 1 {
		m.RecurrenceInterval = 1
	}
	return true
}

// nextMaintenanceWindow 计算当前窗口之后的下一个窗口。
// 传入的 start/end 必须是同一次窗口的时间（保持原有窗口长度）。
// 返回 false 表示周期结束（已达 recurrence_until），调用方应保持 completed。
func nextMaintenanceWindow(m *model.Maintenance, start, end time.Time) (time.Time, time.Time, bool) {
	if !normalizeRecurrence(m) {
		return time.Time{}, time.Time{}, false
	}

	duration := end.Sub(start)
	if duration <= 0 {
		// 结束早于开始的脏数据：给一个 30 分钟的下限，避免产生空窗口
		duration = 30 * time.Minute
	}

	interval := m.RecurrenceInterval
	var nextStart time.Time

	switch m.Recurrence {
	case recurrenceDaily:
		nextStart = start.AddDate(0, 0, interval)

	case recurrenceWeekly:
		nextStart = start.AddDate(0, 0, 7*interval)

	case recurrenceMonthly:
		nextStart = addMonths(start, interval)
		// 指定了「每月第几天」时对齐到该日期（例如每月 1 号 02:00）
		if m.RecurrenceMonthday >= 1 && m.RecurrenceMonthday <= 31 {
			nextStart = time.Date(nextStart.Year(), nextStart.Month(), clampDayOfMonth(nextStart.Year(), nextStart.Month(), m.RecurrenceMonthday),
				start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), start.Location())
		}
	}

	if until, ok := parseUntil(m.RecurrenceUntil); ok && nextStart.After(until) {
		return time.Time{}, time.Time{}, false
	}

	return nextStart, nextStart.Add(duration), true
}

// parseUntil 解析重复截止日期。截止日含当天，因此比较时取当天 23:59:59。
func parseUntil(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			end := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
			return end, true
		}
	}
	return time.Time{}, false
}

// addMonths 按月推进，日期溢出（如 1/31 + 1 月）时收敛到目标月最后一天
func addMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	target := time.Date(y, m, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).AddDate(0, months, 0)
	return time.Date(target.Year(), target.Month(), clampDayOfMonth(target.Year(), target.Month(), d),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// clampDayOfMonth 把「几号」收敛到该月实际存在的最后一天
func clampDayOfMonth(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		return last
	}
	if day < 1 {
		return 1
	}
	return day
}
