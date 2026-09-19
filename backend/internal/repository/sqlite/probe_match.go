package sqlite

import (
	"fmt"
	"strings"
)

// firstItem 取逗号分隔字符串的第 1 段（index 0）
func firstItem(expr string) string {
	return fmt.Sprintf("substr(%s, 1, %s)", expr, itemEnd(expr, 1))
}

// itemEnd 生成「第 n 段（n 从 1 开始）结束位置」的 SQL 片段。
// 该段没有逗号时结束位置就是串长 + 1（substr 允许越界，等价于取到末尾），
// 这样第 1 段与后续分段可以共用同一套写法。
func itemEnd(expr string, n int) string {
	offset := ""
	if n > 1 {
		// 第 n 段的起点：跳过前 n-1 个逗号
		after := fmt.Sprintf("substr(%s, INSTR(%s, ',') + 1)", expr, expr)
		for i := 2; i < n; i++ {
			after = fmt.Sprintf("substr(%s, INSTR(%s, ',') + 1)", after, after)
		}
		offset = after + " || ','"
		expr = after
	}
	return fmt.Sprintf("CASE WHEN INSTR(%s, ',') > 0 THEN INSTR(%s, ',') ELSE LENGTH(%s) + 1 END", expr, expr, offset+expr)
}

// probeStatusOKExpr 生成「心跳行的状态码是否符合该服务期望状态码」的 SQL 表达式。
//
// 判定规则必须与 Go 侧 model.Service.MatchesExpectStatus / ParseExpectStatus 完全一致，
// 否则日志页、延迟分桶、热力图与分位数会各说各话（历史上这四处判定就是各自散落的）。
// 这里按行读取 Service.expect_status，因此筛选「全部服务」时每个服务也能用自己的口径，
// 不需要把服务配置逐条读进内存。
//
// 支持逗号分隔的任意组合：单值（200）、区间（200-299）、通配（2xx / 5xx），
// 例如 "200,301-303,5xx"。所有分支聚合在一条 OR 里，避免为每种写法单独写一个 CASE 分支
// （那正是之前 "200-299,301" 这类组合被判错的原因）。
//
// 约定：serviceAlias 是 Service 表的别名，heartbeatAlias 是 Heartbeat 表的别名。
// 期望状态码由管理端校验后落库，只会包含数字、逗号、连字符与 x，拼接进 SQL 是安全的。
func probeStatusOKExpr(heartbeatAlias, serviceAlias string) string {
	status := column(heartbeatAlias, "status")
	expect := fmt.Sprintf("COALESCE(%s.expect_status, '')", serviceAlias)
	// 统计段数：先去掉所有逗号，再用「原串长度 - 去逗号后长度 + 1」得到段数
	items := fmt.Sprintf("(LENGTH(%s) - LENGTH(REPLACE(%s, ',', '')) + 1)", expect, expect)

	parts := make([]string, 0, 4)

	// 单值写法：200 / 200,301（用带逗号包裹的精确匹配）
	parts = append(parts, fmt.Sprintf(
		"INSTR(',' || %s || ',', ',' || CAST(%s AS TEXT) || ',') > 0", expect, status))

	// 通配写法：2xx（x 大小写不敏感）。只对「单段且形如 2xx」生效。
	parts = append(parts, fmt.Sprintf(
		"(%s = 1 AND UPPER(substr(%s, 2, 1)) = 'X' AND %s >= CAST(substr(%s, 1, 1) AS INTEGER) * 100 AND %s <= CAST(substr(%s, 1, 1) AS INTEGER) * 100 + 99)",
		items, expect, status, expect, status, expect))

	// 区间写法：200-299（单段）或 200-299,301（多段，逐段判断）
	itemRange := func(item string) string {
		return fmt.Sprintf(
			"INSTR(%s, '-') > 0 AND %s >= CAST(substr(%s, 1, INSTR(%s, '-') - 1) AS INTEGER) AND %s <= CAST(substr(%s, INSTR(%s, '-') + 1) AS INTEGER)",
			item, status, item, item, status, item, item)
	}
	parts = append(parts, fmt.Sprintf("(%s = 1 AND %s)", items, itemRange(expect)))
	for i := 2; i <= 4; i++ {
		parts = append(parts, fmt.Sprintf("(%s >= %d AND %s)", items, i, itemRange(firstItem(expect))))
	}

	// 未配置期望状态码时沿用默认判定：2xx/3xx，或 status=1（TCP 连通）
	fallback := fmt.Sprintf("((%s >= 200 AND %s < 400) OR %s = 1)", status, status, status)

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
