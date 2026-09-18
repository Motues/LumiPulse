package http

import (
	"fmt"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetRSSFeed 生成 RSS 2.0 订阅源
func (h *Handler) GetRSSFeed(c *gin.Context) {
	xml, err := h.buildFeed(c, "rss")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}
	c.Header("Content-Type", "application/rss+xml; charset=utf-8")
	c.String(http.StatusOK, xml)
}

// GetAtomFeed 生成 Atom 1.0 订阅源
func (h *Handler) GetAtomFeed(c *gin.Context) {
	xml, err := h.buildFeed(c, "atom")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}
	c.Header("Content-Type", "application/atom+xml; charset=utf-8")
	c.String(http.StatusOK, xml)
}

func (h *Handler) buildFeed(c *gin.Context, feedType string) (string, error) {
	// Determine base URL
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	siteName := utils.GetSetting("site_name")
	if siteName == "" {
		siteName = "LumiPulse"
	}

	// Fetch recent incidents
	incidents, _, err := h.Repo.ListIncidents(c.Request.Context(), 1, 20)
	if err != nil {
		incidents = []*model.Incident{}
	}

	// Fetch updates for all incidents
	incIDs := make([]int64, len(incidents))
	for i, inc := range incidents {
		incIDs[i] = inc.ID
	}
	updatesMap := make(map[int64][]*model.IncidentUpdate)
	if len(incIDs) > 0 {
		updatesMap, _ = h.Repo.BatchListIncidentUpdates(c.Request.Context(), incIDs)
	}

	now := time.Now().UTC().Format(time.RFC1123Z)
	atomNow := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	if feedType == "rss" {
		return h.buildRSS(baseURL, siteName, incidents, updatesMap, now), nil
	}
	return h.buildAtom(baseURL, siteName, incidents, updatesMap, atomNow), nil
}

// parseFeedTime 兼容多种时间格式解析事件时间。
// 事件时间由 time.Now().Format(time.RFC3339) 写入（可能是 +08:00 偏移），
// 早期数据也可能是 UTC 的 …Z，这里统一兼容，避免解析失败后把原始字符串
// 当成 pubDate 输出（那不是合法的 RFC1123，阅读器会解析错误）。
func parseFeedTime(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339,     // 2026-05-20T10:00:00+08:00
		time.RFC3339Nano, // 带纳秒
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (h *Handler) buildRSS(baseURL, siteName string, incidents []*model.Incident, updatesMap map[int64][]*model.IncidentUpdate, now string) string {
	lang := i18n.Current()
	var items strings.Builder
	for _, inc := range incidents {
		title := inc.Title
		st := lang.StatusText(inc.Status)
		desc := fmt.Sprintf("<p><strong>%s:</strong> %s</p><p><strong>%s:</strong> %s</p>",
			lang.FeedStatusLabel(), st, lang.FeedImpactLabel(), lang.ImpactText(inc.Impact))

		if updates, ok := updatesMap[inc.ID]; ok && len(updates) > 0 {
			desc += "<ul>"
			for _, u := range updates {
				if !u.IsInternal {
					desc += fmt.Sprintf("<li><em>%s</em> - %s</li>",
						lang.StatusText(u.Status), escapeXML(u.Content))
				}
			}
			desc += "</ul>"
		}

		pubDate := inc.CreatedAt
		if t, ok := parseFeedTime(inc.CreatedAt); ok {
			pubDate = t.Format(time.RFC1123Z)
		}

		link := fmt.Sprintf("%s/incidents/%s", baseURL, inc.PublicHash)
		guid := fmt.Sprintf("%s/incidents/%s", baseURL, inc.PublicHash)

		items.WriteString(fmt.Sprintf(`    <item>
      <title>%s</title>
      <description><![CDATA[%s]]></description>
      <link>%s</link>
      <guid isPermaLink="true">%s</guid>
      <pubDate>%s</pubDate>
    </item>
`, escapeXML(title), desc, link, guid, pubDate))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>%s</title>
    <link>%s</link>
    <description>%s</description>
    <language>%s</language>
    <lastBuildDate>%s</lastBuildDate>
    <atom:link href="%s/feed/rss" rel="self" type="application/rss+xml"/>
%s  </channel>
</rss>`, escapeXML(lang.FeedTitle(siteName)), baseURL, escapeXML(lang.FeedDescription(siteName)),
		lang.HTMLLang(), now, baseURL, items.String())
}

func (h *Handler) buildAtom(baseURL, siteName string, incidents []*model.Incident, updatesMap map[int64][]*model.IncidentUpdate, now string) string {
	lang := i18n.Current()
	host := hostWithoutPort(strings.TrimPrefix(baseURL, "http://"))
	host = strings.TrimPrefix(host, "https://")
	var entries strings.Builder
	for _, inc := range incidents {
		title := inc.Title
		st := lang.StatusText(inc.Status)
		content := fmt.Sprintf("<p><strong>%s:</strong> %s</p><p><strong>%s:</strong> %s</p>",
			lang.FeedStatusLabel(), st, lang.FeedImpactLabel(), lang.ImpactText(inc.Impact))

		if updates, ok := updatesMap[inc.ID]; ok && len(updates) > 0 {
			content += "<ul>"
			for _, u := range updates {
				if !u.IsInternal {
					content += fmt.Sprintf("<li><em>%s</em> - %s</li>",
						lang.StatusText(u.Status), escapeXML(u.Content))
				}
			}
			content += "</ul>"
		}

		updated := inc.UpdatedAt
		published := inc.CreatedAt
		if t, ok := parseFeedTime(inc.UpdatedAt); ok {
			updated = t.UTC().Format("2006-01-02T15:04:05Z")
		}
		if t, ok := parseFeedTime(inc.CreatedAt); ok {
			published = t.UTC().Format("2006-01-02T15:04:05Z")
		}

		link := fmt.Sprintf("%s/incidents/%s", baseURL, inc.PublicHash)
		id := fmt.Sprintf("tag:%s,%s:/incidents/%s",
			host, inc.CreatedAt[:10], inc.PublicHash)

		entries.WriteString(fmt.Sprintf(`  <entry>
    <title>%s</title>
    <link href="%s" rel="alternate" type="text/html"/>
    <id>%s</id>
    <published>%s</published>
    <updated>%s</updated>
    <content type="html"><![CDATA[%s]]></content>
  </entry>
`, escapeXML(title), link, id, published, updated, content))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="%s">
  <title>%s</title>
  <subtitle>%s</subtitle>
  <link href="%s" rel="alternate" type="text/html"/>
  <link href="%s/feed/atom" rel="self" type="application/atom+xml"/>
  <id>%s/</id>
  <updated>%s</updated>
%s</feed>`, lang.HTMLLang(), escapeXML(lang.FeedTitle(siteName)), escapeXML(lang.FeedDescription(siteName)),
		baseURL, baseURL, baseURL, now, entries.String())
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func hostWithoutPort(host string) string {
	if idx := strings.Index(host, ":"); idx >= 0 {
		return host[:idx]
	}
	return host
}
