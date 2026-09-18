package i18n

import "lumipluse-backend/internal/config"

// Current 返回服务端当前生效的对外内容语言（来自配置文件 LANG）。
// 单独放在这个文件里，让 i18n 的核心只依赖标准库。
func Current() Lang {
	if config.GlobalConfig == nil {
		return Default
	}
	if lang, ok := Normalize(config.GlobalConfig.ContentLang()); ok {
		return lang
	}
	return Default
}

// Resolve 解析一个显式语言值（例如月报邮件的独立语言设置），
// 无法识别时退回 Current。
func Resolve(raw string) Lang {
	if lang, ok := Normalize(raw); ok {
		return lang
	}
	return Current()
}
