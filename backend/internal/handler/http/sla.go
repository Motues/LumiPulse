package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lumipluse-backend/internal/config"
	"lumipluse-backend/internal/model"

	"github.com/gin-gonic/gin"
)

// maxMonthlySLARange 月报/趋势最多可查的月份数。
// 再往前的数据没有任何留存，返回空报告只会让人误以为可用率是 0。
const maxMonthlySLARange = 24

// nowUTCString 返回库内统一使用的 UTC 时间串
func nowUTCString() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// parseMonth 解析 YYYY-MM。空值表示当前月。
func parseMonth(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse("2006-01", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("月份格式应为 YYYY-MM")
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

// monthBounds 返回月份首日、次月首日与月末日（全部 YYYY-MM-DD）
func monthBounds(month time.Time) (from, toExclusive, lastDay string) {
	next := month.AddDate(0, 1, 0)
	return month.Format("2006-01-02"), next.Format("2006-01-02"), next.AddDate(0, 0, -1).Format("2006-01-02")
}

// shiftMonth 按月偏移，offset 为负数表示往前
func shiftMonth(month time.Time, offset int) time.Time {
	return month.AddDate(0, offset, 0)
}

// reportFinal 判断某个月的数据是否已定型：只要月份已经过去，每日明细就不会再新增。
func reportFinal(month, now time.Time) bool {
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return month.Before(current)
}

// SLATrend 返回最近若干个月的整站 SLA 趋势。
func (h *Handler) SLATrend(c *gin.Context) {
	months := 6
	if raw := c.Query("months"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			months = parsed
		}
	}
	if months > maxMonthlySLARange {
		months = maxMonthlySLARange
	}

	now := time.Now().UTC()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	points := make([]*model.SLATrendPoint, 0, months)
	// months 是窗口大小，从最旧的月份开始往前推
	for i := months - 1; i >= 0; i-- {
		month := shiftMonth(currentMonth, -i)
		report, err := h.buildMonthlySLA(c, month)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成趋势数据失败: " + err.Error()})
			return
		}
		points = append(points, &model.SLATrendPoint{
			Month:       report.Month,
			Label:       report.Label,
			Uptime:      report.Summary.Uptime,
			Incidents:   report.Summary.Incidents,
			AvgLatency:  report.Summary.AvgLatency,
			TotalProbes: report.Summary.TotalProbes,
		})
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: model.SLATrendResponse{
			Months:   points,
			Retained: config.GlobalConfig.DailyRetention(),
		},
	})
}

// GetMonthlySLA 返回指定月份的 SLA 报告（默认当前月）。
func (h *Handler) GetMonthlySLA(c *gin.Context) {
	month, err := parseMonth(c.Query("month"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	report, err := h.buildMonthlySLA(c, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成报告失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    report,
	})
}

// buildMonthlySLA 组装单月报告：优先读已固化的月报，缺失时从每日明细实时计算。
func (h *Handler) buildMonthlySLA(c *gin.Context, month time.Time) (*model.MonthlySLAReport, error) {
	return h.buildMonthlySLACtx(c.Request.Context(), month)
}

// buildMonthlySLACtx 与 buildMonthlySLA 相同，但接收 context，
// 便于在非 HTTP 场景（后台归档、测试）复用同一套计算口径。
func (h *Handler) buildMonthlySLACtx(ctx context.Context, month time.Time) (*model.MonthlySLAReport, error) {
	monthKey := month.Format("2006-01")

	services, err := h.Repo.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	// 已固化的月报：月份结束且被归档过，直接复用，避免依赖每日明细
	stored, err := h.Repo.ListServiceMonthlies(ctx, monthKey)
	if err != nil {
		stored = nil
	}

	// 缺失的月份从每日明细实时汇总（历史月份顺便固化下来，保证明细被清理后仍可查）
	missing := make(map[int64]bool)
	for _, svc := range services {
		missing[svc.ID] = true
	}
	for _, m := range stored {
		delete(missing, m.ServiceID)
	}

	var live []*model.ServiceMonthly
	from, toExclusive, _ := monthBounds(month)

	// 覆盖天数只用于提示「该月数据是否完整」，与是否已固化无关，统一实时算一次
	coveredDays, err := h.Repo.CountCoveredDaysBetween(ctx, from, toExclusive)
	if err != nil {
		coveredDays = nil
	}

	if len(missing) > 0 {
		aggregated, err := h.Repo.AggregateServiceMonthly(ctx, from, toExclusive)
		if err != nil {
			return nil, err
		}
		incidentCounts, err := h.Repo.CountIncidentsBetween(ctx, from, toExclusive)
		if err != nil {
			return nil, err
		}
		incidentDowntime, err := h.Repo.AggregateIncidentDowntime(ctx, from, toExclusive)
		if err != nil {
			return nil, err
		}

		for _, agg := range aggregated {
			if !missing[agg.ServiceID] {
				continue
			}
			agg.Month = monthKey
			agg.IncidentTotal = incidentCounts[agg.ServiceID]
			agg.IncidentDowntimeSecs = incidentDowntime[agg.ServiceID]
			live = append(live, agg)
		}

		// 已结束的月份立刻固化，之后即使每日明细被清理也不会丢数据
		if reportFinal(month, time.Now().UTC()) {
			closedAt := nowUTCString()
			for _, m := range live {
				m.ClosedAt = closedAt
				if err := h.Repo.UpsertServiceMonthly(ctx, m, false); err != nil {
					return nil, err
				}
			}
		}
	}

	byService := make(map[int64]*model.ServiceMonthly, len(stored)+len(live))
	for _, m := range stored {
		byService[m.ServiceID] = m
	}
	for _, m := range live {
		byService[m.ServiceID] = m
	}

	report := &model.MonthlySLAReport{
		Month: monthKey,
		Label: month.Format("2006 年 01 月"),
		Days:  daysInMonth(month),
		Final: reportFinal(month, time.Now().UTC()),
	}

	return assembleSLAReport(services, byService, coveredDays, report), nil
}

// assembleSLAReport 把「服务列表 + 每服务聚合 + 覆盖天数」装配成报告。
// 月报与周报共用这一段，避免两处统计口径各自漂移。
func assembleSLAReport(
	services []*model.Service,
	byService map[int64]*model.ServiceMonthly,
	coveredDays map[int64]int,
	report *model.MonthlySLAReport,
) *model.MonthlySLAReport {
	for _, svc := range services {
		summary := &model.ServiceSLASummary{
			ServiceID:  svc.ID,
			PublicHash: svc.PublicHash,
			Name:       svc.Name,
		}

		if m, ok := byService[svc.ID]; ok {
			summary.TotalProbes = m.UptimeCount + m.DowntimeCount
			summary.DowntimeProbes = m.DowntimeCount
			summary.Incidents = m.IncidentTotal
			summary.DowntimeSeconds = m.IncidentDowntimeSecs
			if summary.TotalProbes > 0 {
				summary.Uptime = float64(m.UptimeCount) / float64(summary.TotalProbes) * 100
				summary.AvgLatency = float64(m.TotalLatency) / float64(summary.TotalProbes)
			} else {
				summary.Uptime = 100
			}
		} else {
			// 该周期内没有任何探测数据：可用率给满值，前端据此显示「无数据」
			summary.Uptime = 100
		}

		summary.CoveredDays = coveredDays[svc.ID]

		report.Services = append(report.Services, summary)
		report.Summary.TotalProbes += summary.TotalProbes
		report.Summary.DowntimeProbes += summary.DowntimeProbes
		report.Summary.Incidents += summary.Incidents
		report.Summary.DowntimeSeconds += summary.DowntimeSeconds
		report.Summary.AvgLatency += summary.AvgLatency * float64(summary.TotalProbes)
	}

	if report.Summary.TotalProbes > 0 {
		report.Summary.Uptime = float64(report.Summary.TotalProbes-report.Summary.DowntimeProbes) /
			float64(report.Summary.TotalProbes) * 100
		report.Summary.AvgLatency = report.Summary.AvgLatency / float64(report.Summary.TotalProbes)
	} else {
		report.Summary.Uptime = 100
	}

	return report
}

// daysInMonth 该月天数
func daysInMonth(month time.Time) int {
	return month.AddDate(0, 1, 0).AddDate(0, 0, -1).Day()
}
