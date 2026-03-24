package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"announcementsbot/internal/model"
)

const (
	messageFormatCard    = "card"
	messageFormatMinimal = "minimal"
)

var (
	urlRegex         = regexp.MustCompile(`\bhttps?://[^\s<>()]+`)
	versionRegex     = regexp.MustCompile(`\bv\d+(?:\.\d+)+(?:-[A-Za-z0-9.-]+)?\b`)
	commandLineRegex = regexp.MustCompile(`(?m)^\s*(sudo|apt|yum|dnf|brew|curl|wget|docker|systemctl|kubectl|helm|agave|doublezero)\b.*$`)
)

type Client struct {
	token         string
	timeout       time.Duration
	http          *http.Client
	messageFormat string
}

func New(token string, timeoutMS int, messageFormat string) *Client {
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if messageFormat == "" {
		messageFormat = messageFormatCard
	}
	return &Client{
		token:         token,
		timeout:       timeout,
		http:          &http.Client{Timeout: timeout + 2*time.Second},
		messageFormat: messageFormat,
	}
}

type sendMessageReq struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type sendMessageResp struct {
	OK          bool `json:"ok"`
	Description string
	Result      struct {
		MessageID int `json:"message_id"`
	} `json:"result"`
}

func (c *Client) PublishAnnouncement(ctx context.Context, chatID string, req model.ForwardRequest) (model.ForwardResult, error) {
	text := FormatForwardMessage(req, c.messageFormat)
	return c.PublishText(ctx, chatID, text)
}

func (c *Client) PublishOpsAlert(ctx context.Context, chatID, msg string) (model.ForwardResult, error) {
	return c.PublishText(ctx, chatID, "#ops_alert\n"+msg)
}

func (c *Client) PublishText(ctx context.Context, chatID, text string) (model.ForwardResult, error) {
	formatted := truncate(text, 3900)

	payload := sendMessageReq{
		ChatID:                chatID,
		Text:                  formatted,
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
	}
	buf, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token), bytes.NewReader(buf))
	if err != nil {
		return model.ForwardResult{}, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return model.ForwardResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out sendMessageResp
	if err := json.Unmarshal(body, &out); err != nil {
		return model.ForwardResult{}, fmt.Errorf("telegram invalid response: %w", err)
	}
	if resp.StatusCode >= 300 || !out.OK {
		return model.ForwardResult{}, fmt.Errorf("telegram sendMessage failed: %s", out.Description)
	}

	return model.ForwardResult{ChatID: chatID, MessageID: fmt.Sprintf("%d", out.Result.MessageID)}, nil
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max-20] + "\n\n[truncated]"
}

func FormatForwardMessage(req model.ForwardRequest, messageFormat string) string {
	if messageFormat == messageFormatMinimal {
		return formatForwardMinimal(req)
	}
	return formatForwardCard(req)
}

func FormatEditNotice(req model.EditNoticeRequest) string {
	title := "<b>Updated</b> " + html.EscapeString(req.SourcePrefix)
	if reason := detectEditReason(req.Text); reason != "" {
		title = title + " <i>(" + html.EscapeString(reason) + ")</i>"
	}

	lines := []string{
		title,
		"<b>Channel:</b> " + html.EscapeString(req.ChannelName),
	}

	if formattedTime := formatDisplayTime(req.TimestampISO); formattedTime != "" {
		lines = append(lines, "<b>Time:</b> "+html.EscapeString(formattedTime))
	}
	if req.Permalink != "" {
		lines = append(lines, "<b>Source:</b> "+linkHTML(req.Permalink, "Discord"))
	}
	if context := buildEditContext(req.Text); context != "" {
		lines = append(lines, "", "<b>Context:</b>", context)
	}

	return strings.Join(lines, "\n")
}

func formatForwardCard(req model.ForwardRequest) string {
	category := detectAnnouncementCategory(req.Text)
	lines := []string{
		"<b>New</b> " + html.EscapeString(req.SourcePrefix) + formatCategoryLabel(category),
		"<b>Author:</b> " + html.EscapeString(fallback(req.AuthorName, "Unknown")),
	}

	if formattedTime := formatDisplayTime(req.TimestampISO); formattedTime != "" {
		lines = append(lines, "<b>Time:</b> "+html.EscapeString(formattedTime))
	}
	if req.Permalink != "" {
		lines = append(lines, "<b>Source:</b> "+linkHTML(req.Permalink, "Discord"))
	}

	bodyLines := buildBodySections(req.Text, true)
	if len(bodyLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, bodyLines...)
	}

	if len(req.AttachmentURLs) > 0 {
		lines = append(lines, "", "<b>Attachments:</b>")
		for i, attachmentURL := range req.AttachmentURLs {
			lines = append(lines, fmt.Sprintf("• %s", linkHTML(attachmentURL, attachmentLabel(i))))
		}
	}

	return strings.Join(lines, "\n")
}

func formatForwardMinimal(req model.ForwardRequest) string {
	category := detectAnnouncementCategory(req.Text)
	lines := []string{"<b>New</b> " + html.EscapeString(req.SourcePrefix) + formatCategoryLabel(category)}

	metaParts := []string{}
	if req.AuthorName != "" {
		metaParts = append(metaParts, "By "+html.EscapeString(req.AuthorName))
	}
	if formattedTime := formatDisplayTime(req.TimestampISO); formattedTime != "" {
		metaParts = append(metaParts, html.EscapeString(formattedTime))
	}
	if len(metaParts) > 0 {
		lines = append(lines, strings.Join(metaParts, " • "))
	}

	if text := strings.TrimSpace(req.Text); text != "" {
		lines = append(lines, "")
		lines = append(lines, buildBodySections(text, false)...)
	} else {
		lines = append(lines, "", "<i>[no text content]</i>")
	}

	linkParts := []string{}
	if req.Permalink != "" {
		linkParts = append(linkParts, "<b>Source:</b> "+linkHTML(req.Permalink, "Discord"))
	}
	if len(req.AttachmentURLs) > 0 {
		labels := make([]string, 0, len(req.AttachmentURLs))
		for i, attachmentURL := range req.AttachmentURLs {
			labels = append(labels, linkHTML(attachmentURL, attachmentLabel(i)))
		}
		linkParts = append(linkParts, "<b>Files:</b> "+strings.Join(labels, ", "))
	}
	if len(linkParts) > 0 {
		lines = append(lines, "", strings.Join(linkParts, " | "))
	}

	return strings.Join(lines, "\n")
}

func buildBodySections(text string, splitCommands bool) []string {
	body := strings.TrimSpace(text)
	if body == "" {
		return []string{"<i>[no text content]</i>"}
	}

	command := ""
	if splitCommands {
		body, command = extractCommand(body)
	}

	lines := []string{"<b>Summary:</b>", formatInlineText(body)}
	if command != "" {
		lines = append(lines, "", "<b>Command:</b>", "<code>"+html.EscapeString(command)+"</code>")
	}
	return lines
}

func extractCommand(text string) (string, string) {
	match := commandLineRegex.FindString(text)
	if match == "" {
		return text, ""
	}
	trimmed := strings.TrimSpace(match)
	cleaned := strings.TrimSpace(strings.Replace(text, match, "", 1))
	return cleaned, trimmed
}

func buildEditContext(text string) string {
	body := strings.TrimSpace(text)
	if body == "" {
		return ""
	}

	for _, line := range strings.Split(body, "\n") {
		candidate := strings.TrimSpace(line)
		if candidate != "" {
			return shortenHTML(formatInlineText(candidate), 220)
		}
	}
	return ""
}

func formatInlineText(text string) string {
	trimmed := strings.TrimSpace(text)
	matches := urlRegex.FindAllStringIndex(trimmed, -1)
	if len(matches) == 0 {
		return formatRichNonURLText(trimmed)
	}

	var b strings.Builder
	cursor := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start > cursor {
			b.WriteString(formatRichNonURLText(trimmed[cursor:start]))
		}
		url := trimmed[start:end]
		b.WriteString(linkHTML(url, url))
		cursor = end
	}
	if cursor < len(trimmed) {
		b.WriteString(formatRichNonURLText(trimmed[cursor:]))
	}
	return b.String()
}

func highlightVersions(text string) string {
	return versionRegex.ReplaceAllStringFunc(text, func(match string) string {
		return "<code>" + match + "</code>"
	})
}

func formatRichNonURLText(text string) string {
	if strings.TrimSpace(text) == "" {
		return html.EscapeString(text)
	}

	var b strings.Builder
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "```"):
			end := strings.Index(text[i+3:], "```")
			if end < 0 {
				b.WriteString(formatPlainRichText(text[i:]))
				return b.String()
			}
			code := strings.Trim(text[i+3:i+3+end], "\n")
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
				b.WriteString("\n")
			}
			b.WriteString("<pre><code>")
			b.WriteString(html.EscapeString(code))
			b.WriteString("</code></pre>")
			i += 3 + end + 3
		case text[i] == '`':
			end := strings.IndexByte(text[i+1:], '`')
			if end < 0 {
				b.WriteString(formatPlainRichText(text[i:]))
				return b.String()
			}
			code := text[i+1 : i+1+end]
			b.WriteString("<code>")
			b.WriteString(html.EscapeString(code))
			b.WriteString("</code>")
			i += end + 2
		default:
			next := nextSpecialIndex(text[i:])
			if next < 0 {
				b.WriteString(formatPlainRichText(text[i:]))
				return b.String()
			}
			b.WriteString(formatPlainRichText(text[i : i+next]))
			i += next
		}
	}

	return b.String()
}

func formatPlainRichText(text string) string {
	escaped := html.EscapeString(text)
	escaped = highlightBracketedText(escaped)
	return highlightVersions(escaped)
}

func highlightBracketedText(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '[' {
			b.WriteByte(text[i])
			i++
			continue
		}

		end := strings.IndexByte(text[i+1:], ']')
		if end < 0 {
			b.WriteByte(text[i])
			i++
			continue
		}

		content := text[i : i+end+2]
		b.WriteString("<b>")
		b.WriteString(content)
		b.WriteString("</b>")
		i += end + 2
	}
	return b.String()
}

func nextSpecialIndex(text string) int {
	indexes := []int{}
	if idx := strings.Index(text, "```"); idx >= 0 {
		indexes = append(indexes, idx)
	}
	if idx := strings.IndexByte(text, '`'); idx >= 0 {
		indexes = append(indexes, idx)
	}
	if len(indexes) == 0 {
		return -1
	}

	min := indexes[0]
	for _, idx := range indexes[1:] {
		if idx < min {
			min = idx
		}
	}
	return min
}

func detectAnnouncementCategory(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(normalized, "activate", "activation", "epoch", "feature gate", "version floor"):
		return "Activation"
	case containsAny(normalized, "warning", "forked", "unsupported version", "critical", "urgent"):
		return "Warning"
	case containsAny(normalized, "maintenance", "outage", "downtime"):
		return "Maintenance"
	case containsAny(normalized, "upgrade", "recommended version", "please upgrade"):
		return "Upgrade"
	case containsAny(normalized, "release", "recommended for use", "recommended for general use", "changelog"):
		return "Release"
	default:
		return ""
	}
}

func detectEditReason(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(normalized, "recommended version", "recommended for use", "recommended for general use"):
		return "recommended version changed"
	case containsAny(normalized, "version floor", "unsupported version", "forked"):
		return "version floor changed"
	case containsAny(normalized, "activate", "activation", "epoch", "feature gate"):
		return "activation details changed"
	case containsAny(normalized, "upgrade", "please upgrade"):
		return "upgrade guidance changed"
	case containsAny(normalized, "maintenance", "outage", "downtime"):
		return "maintenance details changed"
	default:
		return "announcement updated"
	}
}

func formatCategoryLabel(category string) string {
	if category == "" {
		return ""
	}
	return " <i>(" + html.EscapeString(category) + ")</i>"
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func linkHTML(url, label string) string {
	escapedURL := html.EscapeString(url)
	escapedLabel := html.EscapeString(label)
	return fmt.Sprintf(`<a href="%s">%s</a>`, escapedURL, escapedLabel)
}

func attachmentLabel(index int) string {
	return fmt.Sprintf("File %d", index+1)
}

func formatDisplayTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		ts, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return value
		}
	}
	return ts.UTC().Format("2006-01-02 15:04 UTC")
}

func shortenHTML(input string, max int) string {
	if len(input) <= max {
		return input
	}
	cut := input[:max]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return strings.TrimSpace(cut) + "…"
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}
