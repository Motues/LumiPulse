package sqlite

import (
	"fmt"
	"strings"
)

// maxStatusPatternItems SQL 判定里「按位置逐段解析」的段数上限。
//
// 段表达式是嵌套 substr（每层都要重复上一层的结果才能定位下一个逗号），
// 大小随段数指数增长，因此这里限制 4 段，与历史实现的上限保持一致。
// 注意：单值写法（200,301,404…）不依赖逐段解析——它们由整体的 INSTR 精确
// 匹配覆盖，段数不受此限制；只有第 5 段之后的「区间 / 通配」不会参与 SQL 判定
// （Go 侧探测不受影响，界面保存时也极少出现）。
const maxStatusPatternItems = 4

// itemAt 生成「逗号分隔串的第 n 段（n 从 1 开始）」的子串表达式（已 TRIM）。
// 第 1 段从位置 1 开始；之后每段都从「上一个逗号之后」开始，
// 长度由该段内是否还有逗号决定（有逗号就取到逗号前，否则取到末尾）。
func itemAt(expr string, n int) string {
	start := expr
	for i := 1; i < n; i++ {
		start = fmt.Sprintf("substr(%s, INSTR(%s, ',') + 1)", start, start)
	}
	return fmt.Sprintf(
		"TRIM(substr(%s, 1, CASE WHEN INSTR(%s, ',') > 0 THEN INSTR(%s, ',') - 1 ELSE LENGTH(%s) END))",
		start, start, start, start)
}

// itemMatchExpr 生成「某一段期望状态码是否命中当前 status」的 SQL 表达式。
// 只处理区间（200-299）与通配（2xx）两种写法：单值已经由整体 INSTR 精确匹配覆盖。
// 非法写法在保存时就会被拒绝，这里 CAST 失败会得到 0，天然不命中。
func itemMatchExpr(item, status string) string {
	return fmt.Sprintf(`(CASE
		WHEN INSTR(%[1]s, '-') > 0 THEN
			CAST(%[2]s AS INTEGER) >= CAST(substr(%[1]s, 1, INSTR(%[1]s, '-') - 1) AS INTEGER)
			AND CAST(%[2]s AS INTEGER) <= CAST(substr(%[1]s, INSTR(%[1]s, '-') + 1) AS INTEGER)
		WHEN LENGTH(%[1]s) = 3 AND UPPER(substr(%[1]s, 2, 1)) = 'X' AND UPPER(substr(%[1]s, 3, 1)) = 'X' THEN
			CAST(%[2]s AS INTEGER) >= CAST(substr(%[1]s, 1, 1) AS INTEGER) * 100
			AND CAST(%[2]s AS INTEGER) < (CAST(substr(%[1]s, 1, 1) AS INTEGER) + 1) * 100
		ELSE 0
	END)`, item, status)
}

// probeStatusOKExpr 生成「心跳行的状态码是否符合该服务期望状态码」的 SQL 表达式。
//
// 判定规则必须与 Go 侧 model.Service.MatchesExpectStatus / ParseExpectStatus 完全一致，
// 否则日志页、延迟分桶、热力图与分位数会各说各话（历史上这四处判定就是各自散落的）。
// 这里按行读取 Service.expect_status，因此筛选「全部服务」时每个服务也能用自己的口径，
// 不需要把服务配置逐条读进内存。
//
// 支持逗号分隔的任意组合：单值（200）、区间（200-299）、通配（2xx / 5xx），
// 例如 "200,301-303,5xx"。所有段都要参与判断：早期版本在循环里始终取第 1 段，
// 导致 "200,301-303,5xx" 里的区间与通配全部失效（301/500 被当成失败）。
//
// 约定：serviceAlias 是 Service 表的别名，heartbeatAlias 是 Heartbeat 表的别名。
// 期望状态码由管理端校验后落库，只会包含数字、逗号、连字符与 x，拼接进 SQL 是安全的。
func probeStatusOKExpr(heartbeatAlias, serviceAlias string) string {
	status := fmt.Sprintf("CAST(%s AS INTEGER)", column(heartbeatAlias, "status"))
	rawStatus := column(heartbeatAlias, "status")
	expect := fmt.Sprintf("COALESCE(%s.expect_status, '')", serviceAlias)

	// 单值写法：200 / 200,301（用带逗号包裹的精确匹配，一次覆盖所有段）
	parts := []string{
		fmt.Sprintf("INSTR(',' || %s || ',', ',' || CAST(%s AS TEXT) || ',') > 0", expect, rawStatus),
	}
	// 区间与通配：逐段判断
	for i := 1; i <= maxStatusPatternItems; i++ {
		parts = append(parts, itemMatchExpr(itemAt(expect, i), status))
	}

	// 未配置期望状态码时沿用默认判定：2xx/3xx，或 status=1（TCP 连通）
	fallback := fmt.Sprintf("((%s >= 200 AND %s < 400) OR %s = 1)", rawStatus, rawStatus, rawStatus)

	return fmt.Sprintf(
		"(CASE WHEN TRIM(%s) = '' THEN %s ELSE (%s) END)",
		expect, fallback, strings.Join(parts, " OR "))
}

// probeOKExpr 生成某条心跳「本次探测是否成功」的完整判定：
// 状态码符合期望，且在配置了期望关键字时响应片段包含该关键字。
// 两个条件都与 Go 侧 model.Service.MatchesProbe 对齐。
func probeOKExpr(heartbeatAlias, serviceAlias string) string {
	expr := probeStatusOKExpr(heartbeatAlias, serviceAlias)
	keyword := fmt.Sprintf("TRIM(COALESCE(%s.expect_keyword, ''))", serviceAlias)
	message := column(heartbeatAlias, "message")
	return fmt.Sprintf("(%s AND (%s = '' OR INSTR(COALESCE(%s, ''), %s) > 0))", expr, keyword, message, keyword)
}

// column 拼出带表别名的列名；别名为空时直接返回列名。
func column(alias, name string) string {
	if strings.TrimSpace(alias) == "" {
		return name
	}
	return alias + "." + name
}

// serviceJoin 统计查询统一使用的 Service 关联，别名固定为 s。
// 用 LEFT JOIN 是为了保留心跳但服务已被删除的历史数据（此时按默认判定处理）。
const serviceJoin = `LEFT JOIN Service s ON s.id = h.service_id`
