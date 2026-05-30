package http

import (
	"fmt"
	"lumipluse-backend/internal/model"
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

func (h *Handler) buildRSS(baseURL, siteName string, incidents []*model.Incident, updatesMap map[int64][]*model.IncidentUpdate, now string) string {
	var items strings.Builder
	for _, inc := range incidents {
		title := inc.Title
		st := statusText(inc.Status)
		desc := fmt.Sprintf("<p><strong>状态:</strong> %s</p><p><strong>影响:</strong> %s</p>",
			st, impactTextCN(inc.Impact))

		if updates, ok := updatesMap[inc.ID]; ok && len(updates) > 0 {
			desc += "<ul>"
			for _, u := range updates {
				if !u.IsInternal {
					desc += fmt.Sprintf("<li><em>%s</em> - %s</li>",
						st, escapeXML(u.Content))
				}
			}
			desc += "</ul>"
		}

		pubDate := inc.CreatedAt
		if t, err := time.Parse("2006-01-02T15:04:05Z", inc.CreatedAt); err == nil {
			pubDate = t.Format(time.RFC1123Z)
		}

		link := fmt.Sprintf("%s/incidents/%d", baseURL, inc.ID)
		guid := fmt.Sprintf("%s/incidents/%d", baseURL, inc.ID)

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
    <title>%s - 状态更新</title>
    <link>%s</link>
    <description>%s 系统状态与故障事件更新</description>
    <language>zh-CN</language>
    <lastBuildDate>%s</lastBuildDate>
    <atom:link href="%s/api/v1/feed/rss" rel="self" type="application/rss+xml"/>
%s  </channel>
</rss>`, siteName, baseURL, siteName, now, baseURL, items.String())
}

func (h *Handler) buildAtom(baseURL, siteName string, incidents []*model.Incident, updatesMap map[int64][]*model.IncidentUpdate, now string) string {
	host := hostWithoutPort(strings.TrimPrefix(baseURL, "http://"))
	host = strings.TrimPrefix(host, "https://")
	var entries strings.Builder
	for _, inc := range incidents {
		title := inc.Title
		st := statusText(inc.Status)
		content := fmt.Sprintf("<p><strong>状态:</strong> %s</p><p><strong>影响:</strong> %s</p>",
			st, impactTextCN(inc.Impact))

		if updates, ok := updatesMap[inc.ID]; ok && len(updates) > 0 {
			content += "<ul>"
			for _, u := range updates {
				if !u.IsInternal {
					content += fmt.Sprintf("<li><em>%s</em> - %s</li>",
						st, escapeXML(u.Content))
				}
			}
			content += "</ul>"
		}

		updated := inc.UpdatedAt
		published := inc.CreatedAt
		if t, err := time.Parse("2006-01-02T15:04:05Z", inc.UpdatedAt); err == nil {
			updated = t.Format("2006-01-02T15:04:05Z")
		}
		if t, err := time.Parse("2006-01-02T15:04:05Z", inc.CreatedAt); err == nil {
			published = t.Format("2006-01-02T15:04:05Z")
		}

		link := fmt.Sprintf("%s/incidents/%d", baseURL, inc.ID)
		id := fmt.Sprintf("tag:%s,%s:/incidents/%d",
			host, inc.CreatedAt[:10], inc.ID)

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
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>%s - 状态更新</title>
  <subtitle>%s 系统状态与故障事件更新</subtitle>
  <link href="%s" rel="alternate" type="text/html"/>
  <link href="%s/api/v1/feed/atom" rel="self" type="application/atom+xml"/>
  <id>%s/</id>
  <updated>%s</updated>
%s</feed>`, siteName, siteName, baseURL, baseURL, baseURL, now, entries.String())
}

func statusText(s string) string {
	switch s {
	case "investigating":
		return "调查中"
	case "identified":
		return "已确认"
	case "monitoring":
		return "监控中"
	case "resolved":
		return "已解决"
	default:
		return s
	}
}

func impactTextCN(s string) string {
	switch s {
	case "minor":
		return "轻微"
	case "major":
		return "重大"
	case "critical":
		return "严重"
	default:
		return s
	}
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
