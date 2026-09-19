package http

import (
	"net/http"
	"strings"

	"lumipluse-backend/internal/pkg/utils"

	"github.com/gin-gonic/gin"
)

// 公开页可被收录的路径。服务详情与事件详情都用随机 hash 定位，
// 无法离线枚举，只能从数据库里取出来交给爬虫。
const sitemapIncidentLimit = 50

// publicBaseURL 推断站点对外可访问的根地址。
// 部署在反向代理后时，Host 与协议由代理头决定；两者都缺失时退化为相对根路径。
func publicBaseURL(c *gin.Context) string {
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return ""
	}

	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	// X-Forwarded-Host 可能是逗号分隔的链，取最外层
	if idx := strings.IndexByte(host, ','); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	return scheme + "://" + host
}

// siteDisplayName 站点名，未配置时用产品默认名兜底
func siteDisplayName() string {
	if name := strings.TrimSpace(utils.GetSetting("site_name")); name != "" {
		return name
	}
	return "LumiPulse"
}

// GetRobotsTxt 输出 robots.txt。允许收录公开状态页，屏蔽管理后台与接口。
func (h *Handler) GetRobotsTxt(c *gin.Context) {
	base := publicBaseURL(c)

	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	b.WriteString("Disallow: /api/\n")
	b.WriteString("Disallow: /admin\n")
	b.WriteString("Disallow: /login\n")
	b.WriteString("Disallow: /setup\n")
	// 退订页带签名令牌，属于个人链接，不应被收录
	b.WriteString("Disallow: /unsubscribe\n")
	if base != "" {
		b.WriteString("\nSitemap: " + base + "/sitemap.xml\n")
	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}

// GetSitemap 输出 sitemap.xml：首页 + 公开服务详情 + 近期事件详情。
func (h *Handler) GetSitemap(c *gin.Context) {
	base := publicBaseURL(c)
	ctx := c.Request.Context()

	type urlEntry struct {
		loc     string
		lastmod string
	}

	entries := []urlEntry{{loc: base + "/"}}

	if services, err := h.Repo.ListServices(ctx); err == nil {
		for _, svc := range services {
			// 只在首页展示的服务才值得收录
			if !svc.IsActive || !svc.ShowOnHomepage || svc.PublicHash == "" {
				continue
			}
			entries = append(entries, urlEntry{
				loc:     base + "/services/" + svc.PublicHash,
				lastmod: isoDate(svc.UpdatedAt),
			})
		}
	}

	if incidents, _, err := h.Repo.ListIncidents(ctx, 1, sitemapIncidentLimit); err == nil {
		for _, inc := range incidents {
			if inc.PublicHash == "" {
				continue
			}
			entries = append(entries, urlEntry{
				loc:     base + "/incidents/" + inc.PublicHash,
				lastmod: isoDate(inc.UpdatedAt),
			})
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, e := range entries {
		b.WriteString("  <url>\n")
		// escapeXML 复用 feed.go 中的实现（RSS / Atom / sitemap 共用同一套转义）
		b.WriteString("    <loc>" + escapeXML(e.loc) + "</loc>\n")
		if e.lastmod != "" {
			b.WriteString("    <lastmod>" + e.lastmod + "</lastmod>\n")
		}
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")

	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(b.String()))
}

// isoDate 把库里存储的时间串裁剪成 sitemap 需要的 YYYY-MM-DD
func isoDate(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	return ts[:10]
}
