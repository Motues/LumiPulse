package http

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// badgeCacheSeconds 徽章响应的缓存时长。状态变化不会秒级生效，
// 且 README / 文档站会频繁拉取，因此允许客户端与 CDN 短时缓存。
const badgeCacheSeconds = 60

// badgeStatusStyle 徽章右侧状态文案与配色。文案保持英文常量，
// 与 shields.io 的风格一致，也不受 LANG 配置影响（README 里通常中英混排）。
type badgeStatusStyle struct {
	text  string
	color string
}

var badgeStatusStyles = map[string]badgeStatusStyle{
	"operational": {text: "operational", color: "#34a761"},
	"degraded":    {text: "degraded", color: "#fda305"},
	"outage":      {text: "outage", color: "#df2d2a"},
}

// badgeCharWidth 单个字符的估算宽度（SVG 内部按 110 号字缩放，这里是缩放后的逻辑宽度）。
const badgeCharWidth = 7

// GetStatusBadge 对外状态徽章：GET /badge/:hash.svg
//
// 无需鉴权，返回 shields.io 风格的 SVG，可直接嵌入 README / 文档站：
//
//	![](https://status.example.com/badge/<publicHash>.svg)
//
// 路由用 :file 捕获整段（gin 不支持「参数 + 后缀」的同一路径段），
// 因此这里手动校验 .svg 后缀。
func (h *Handler) GetStatusBadge(c *gin.Context) {
	file := strings.TrimSpace(c.Param("file"))
	if !strings.HasSuffix(file, ".svg") {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not found"})
		return
	}
	hash := strings.TrimSuffix(file, ".svg")
	if hash == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not found"})
		return
	}

	svc, err := h.Repo.GetServiceByHash(c.Request.Context(), hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Service not found"})
		return
	}

	// 与公开状态页保持同一判定：事件状态会覆盖服务自身的 status
	status := svc.Status
	if incidents, err := h.Repo.ListActiveIncidents(c.Request.Context()); err == nil {
		status = reconcileStatus(incidents, svc.ID)
	}

	style, ok := badgeStatusStyles[status]
	if !ok {
		style = badgeStatusStyles["operational"]
	}

	svg := renderBadgeSVG("status", style.text, style.color)

	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", badgeCacheSeconds))
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(svg))
}

// renderBadgeSVG 生成 flat 风格的徽章。宽度按字符数估算，
// 不依赖具体字体，因此无需在服务端做文本测量。
func renderBadgeSVG(label, value, color string) string {
	labelW := len(label)*badgeCharWidth + 10
	valueW := len(value)*badgeCharWidth + 10
	totalW := labelW + valueW
	labelX := labelW * 5
	valueX := (labelW + valueW/2) * 10

	escLabel := html.EscapeString(label)
	escValue := html.EscapeString(value)
	escAria := html.EscapeString(label + ": " + value)

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s">
  <title>%s</title>
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="%d" height="20" fill="#555"/>
    <rect x="%d" width="%d" height="20" fill="%s"/>
    <rect width="%d" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">
    <text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)">%s</text>
    <text x="%d" y="140" transform="scale(.1)">%s</text>
    <text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)">%s</text>
    <text x="%d" y="140" transform="scale(.1)">%s</text>
  </g>
</svg>`,
		totalW, escAria, escAria,
		totalW,
		labelW,
		labelW, valueW, color,
		totalW,
		labelX, escLabel,
		labelX, escLabel,
		valueX, escValue,
		valueX, escValue,
	)
}
