package processor

import (
	"strings"

	"announcementsbot/internal/model"
)

func evaluateKeywords(event *model.AnnouncementEvent, channel model.ChannelConfig) model.FilterDecision {
	haystack := strings.ToLower(event.ContentText + "\n" + event.EmbedText)
	if len(channel.Keywords) == 0 {
		return model.FilterDecision{Matched: true, MatchedKeywords: []string{}}
	}
	matched := make([]string, 0, len(channel.Keywords))
	for _, keyword := range channel.Keywords {
		k := strings.ToLower(strings.TrimSpace(keyword))
		if k == "" {
			continue
		}
		if strings.Contains(haystack, k) {
			matched = append(matched, k)
		}
	}
	return model.FilterDecision{Matched: len(matched) > 0, MatchedKeywords: matched}
}

// TestEvaluateKeywords allows external tests to verify filter behavior against the production logic.
func TestEvaluateKeywords(event *model.AnnouncementEvent, channel model.ChannelConfig) model.FilterDecision {
	return evaluateKeywords(event, channel)
}
