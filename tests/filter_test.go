package tests

import (
	"testing"

	"announcementsbot/internal/model"
	"announcementsbot/internal/processor"
)

func TestKeywordFilterCaseInsensitive(t *testing.T) {
	event := &model.AnnouncementEvent{ContentText: "Mainnet upgrade scheduled", EmbedText: ""}
	channel := model.ChannelConfig{ID: "c", Name: "mainnet-anns", Enabled: true, Keywords: []string{"UpGrAdE", "maintenance"}}

	result := processor.TestEvaluateKeywords(event, channel)
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if len(result.MatchedKeywords) != 1 || result.MatchedKeywords[0] != "upgrade" {
		t.Fatalf("unexpected matched keywords: %#v", result.MatchedKeywords)
	}
}

func TestKeywordFilterEmptyMeansForwardAll(t *testing.T) {
	event := &model.AnnouncementEvent{ContentText: "anything", EmbedText: ""}
	channel := model.ChannelConfig{ID: "c", Name: "testnet-anns", Enabled: true, Keywords: []string{}}

	result := processor.TestEvaluateKeywords(event, channel)
	if !result.Matched {
		t.Fatalf("expected match for empty keyword list")
	}
}
