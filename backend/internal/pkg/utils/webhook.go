package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Webhook 通知渠道。渠道类型决定请求体的形状：
// 只有 generic 会带 HMAC 签名，其余三个（Slack / Discord / Telegram）本身就是专用协议。
const (
	WebhookTypeGeneric  = "generic"
	WebhookTypeSlack    = "slack"
	WebhookTypeDiscord  = "discord"
	WebhookTypeTelegram = "telegram"
)

// Webhook 事件类型
const (
	WebhookEventDown = "down" // 服务异常（自动创建事件）
	WebhookEventUp   = "up"   // 服务恢复
)

// 签名相关请求头。时间戳单独放在头里，接收端可据此拒绝重放请求。
const (
	WebhookSignatureHeader = "X-LumiPulse-Signature"
	WebhookTimestampHeader = "X-LumiPulse-Timestamp"
	WebhookEventHeader     = "X-LumiPulse-Event"
)

// WebhookAttemptTimeout 单次投递超时。webhook 在探测协程里同步发送，
// 必须设上限，否则接收端不响应会拖慢整个探测循环。
const WebhookAttemptTimeout = 10 * time.Second

// WebhookEvent webhook 通知载荷中的事件内容
type WebhookEvent struct {
	Event   string // down / up
	Service string // 服务名
	URL     string // 服务地址（可为空）
	Time    string // 事件时间（RFC3339）
	Message string // 面向人的描述
}

// SendWebhookNotifier 统一入口：按设置决定是否发送、能否发送，并选择渠道格式。
// 未启用、未选中该事件类型时静默返回 nil（这不是错误，只是没配置）。
func SendWebhookNotifier(evt WebhookEvent) error {
	if GetSetting("webhook_enabled") != "true" {
		return nil
	}
	if !webhookEventEnabled(evt.Event) {
		return nil
	}
	return SendWebhookNow(evt)
}

// SendWebhookNow 只做投递，不检查开关与事件订阅。
// 管理端的「发送测试」需要绕过开关，直接用当前配置发一条，因此单独导出。
func SendWebhookNow(evt WebhookEvent) error {
	webhookType := strings.TrimSpace(strings.ToLower(GetSetting("webhook_type")))
	switch webhookType {
	case WebhookTypeSlack, WebhookTypeDiscord, WebhookTypeTelegram:
	default:
		webhookType = WebhookTypeGeneric
	}

	url := strings.TrimSpace(GetSetting("webhook_url"))
	if url == "" {
		return fmt.Errorf("webhook 地址未配置")
	}
	secret := GetSetting("webhook_secret")

	payload, err := buildWebhookPayload(webhookType, evt)
	if err != nil {
		return err
	}

	headers := map[string]string{
		WebhookEventHeader: evt.Event,
	}
	// 通用 webhook 支持 HMAC-SHA256 签名，接收端可据此校验来源与完整性
	if webhookType == WebhookTypeGeneric && secret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		headers[WebhookTimestampHeader] = ts
		headers[WebhookSignatureHeader] = "sha256=" + SignWebhookPayload(secret, ts, payload)
	}

	return postWebhook(url, payload, headers)
}

// SignWebhookPayload 计算签名：HMAC-SHA256(secret, timestamp + "." + body)。
// 把时间戳一并纳入签名，接收端校验时间窗口即可防重放。
func SignWebhookPayload(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// webhookEventEnabled 解析 webhook_events（逗号分隔）。未配置视为两种事件都订阅。
func webhookEventEnabled(event string) bool {
	raw := strings.TrimSpace(GetSetting("webhook_events"))
	if raw == "" {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == event {
			return true
		}
	}
	return false
}

// webhookColor 按事件类型给出强调色，Slack / Discord 都用它渲染左侧色条
func webhookColor(event string) int {
	if event == WebhookEventUp {
		return 0x34a761 // 绿色：恢复
	}
	return 0xdf2d2a // 红色：异常
}

// webhookTitle 事件标题，例如「服务异常告警: API」
func webhookTitle(evt WebhookEvent) string {
	if evt.Event == WebhookEventUp {
		return fmt.Sprintf("服务恢复通知: %s", evt.Service)
	}
	return fmt.Sprintf("服务异常告警: %s", evt.Service)
}

// buildWebhookPayload 按渠道生成请求体
func buildWebhookPayload(webhookType string, evt WebhookEvent) ([]byte, error) {
	title := webhookTitle(evt)

	switch webhookType {
	case WebhookTypeSlack:
		// Slack Incoming Webhook：用 attachments 才能带颜色与字段
		fields := []map[string]any{{"title": "服务", "value": evt.Service, "short": true}}
		if evt.URL != "" {
			fields = append(fields, map[string]any{"title": "地址", "value": evt.URL, "short": true})
		}
		fields = append(fields, map[string]any{"title": "时间", "value": evt.Time, "short": false})

		return json.Marshal(map[string]any{
			"text": title,
			"attachments": []map[string]any{{
				"color":     fmt.Sprintf("#%06x", webhookColor(evt.Event)),
				"title":     title,
				"text":      evt.Message,
				"fields":    fields,
				"footer":    "LumiPulse",
				"ts":        time.Now().Unix(),
				"mrkdwn_in": []string{"text"},
			}},
		})

	case WebhookTypeDiscord:
		// Discord Webhook：embeds
		fields := []map[string]any{{"name": "服务", "value": evt.Service, "inline": true}}
		if evt.URL != "" {
			fields = append(fields, map[string]any{"name": "地址", "value": evt.URL, "inline": true})
		}

		return json.Marshal(map[string]any{
			"embeds": []map[string]any{{
				"title":       title,
				"description": evt.Message,
				"color":       webhookColor(evt.Event),
				"timestamp":   evt.Time,
				"footer":      map[string]any{"text": "LumiPulse"},
				"fields":      fields,
			}},
		})

	case WebhookTypeTelegram:
		// Telegram Bot API：token 直接写在 URL 里（https://api.telegram.org/bot<token>/sendMessage）
		lines := []string{"*" + title + "*", evt.Message, "服务: " + evt.Service}
		if evt.URL != "" {
			lines = append(lines, "地址: "+evt.URL)
		}
		lines = append(lines, "时间: "+evt.Time)

		return json.Marshal(map[string]any{
			"chat_id":    GetSetting("webhook_telegram_chat_id"),
			"text":       strings.Join(lines, "\n"),
			"parse_mode": "Markdown",
		})

	default:
		// 通用 webhook：接收端自己解析，字段语义保持稳定
		return json.Marshal(map[string]any{
			"event":     evt.Event,
			"service":   evt.Service,
			"url":       evt.URL,
			"message":   evt.Message,
			"timestamp": evt.Time,
		})
	}
}

// postWebhook 发送请求。返回体只读取少量字节用于错误提示，避免大响应体占内存。
func postWebhook(url string, payload []byte, headers map[string]string) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LumiPulse")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: WebhookAttemptTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return fmt.Errorf("接收端返回 %d", resp.StatusCode)
		}
		return fmt.Errorf("接收端返回 %d: %s", resp.StatusCode, detail)
	}
	return nil
}
