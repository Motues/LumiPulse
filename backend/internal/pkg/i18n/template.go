package i18n

import (
	"regexp"
	"strings"
)

// 通知模板渲染。
//
// 模板由管理员在「通知管理」里填写，留空时使用内置的中/英文案（本包的
// Alert* / Resolved* 等方法）。这里只做纯文本变量替换：
//   - 变量写成 {{name}}，两侧空格可有可无；
//   - 已知变量按其值替换（值为空就替换成空）；
//   - 未知变量原样保留，方便管理员发现自己拼错了变量名。
//
// HTML 不属于模板的职责：邮件外壳会在写入内容时统一做转义与换行处理，
// 因此同一份模板既能用于邮件正文，也能原样用作 webhook 的 message。

var templateVarPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// RenderTemplate 按变量表渲染模板
func RenderTemplate(tpl string, vars map[string]string) string {
	if tpl == "" {
		return ""
	}
	return templateVarPattern.ReplaceAllStringFunc(tpl, func(match string) string {
		name := templateVarPattern.FindStringSubmatch(match)[1]
		if value, ok := vars[name]; ok {
			return value
		}
		return match
	})
}

// TemplateVariables 返回模板支持的变量名（用于设置页的说明，按事件类型分组）。
// 保留在服务端是为了让文档与实现不至于各写一份。
func TemplateVariables(event string) []string {
	switch event {
	case "alert":
		return []string{"service", "url", "time", "site"}
	case "resolved":
		return []string{"service", "url", "time", "duration", "site"}
	case "maintenance":
		return []string{"title", "start", "countdown", "site"}
	case "cert":
		return []string{"service", "expires", "remaining", "site"}
	default:
		return []string{"service", "site"}
	}
}

// TemplateVariableHint 变量说明文案（设置页直接展示）
func TemplateVariableHint(event string) string {
	desc := map[string]string{
		"service":   "服务名称",
		"url":       "服务地址",
		"time":      "事件时间（北京时间）",
		"duration":  "本次不可用时长",
		"title":     "维护计划标题",
		"start":     "维护开始时间（北京时间）",
		"countdown": "距维护开始的剩余时长",
		"expires":   "证书到期时间（北京时间）",
		"remaining": "证书剩余可用时长",
		"site":      "站点名称",
	}
	parts := make([]string, 0, len(TemplateVariables(event)))
	for _, name := range TemplateVariables(event) {
		parts = append(parts, "{{"+name+"}} "+desc[name])
	}
	return strings.Join(parts, "、")
}
