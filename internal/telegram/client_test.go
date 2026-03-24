package telegram

import (
	"strings"
	"testing"

	"announcementsbot/internal/model"
)

func TestFormatForwardMessageCardUsesClickableSourceAndReadableTime(t *testing.T) {
	msg := FormatForwardMessage(model.ForwardRequest{
		SourcePrefix: "[solana-tds]",
		Theme:        "green",
		TitleBold:    true,
		AuthorName:   "Solana Tech",
		TimestampISO: "2026-03-24T06:39:47.113Z",
		Text:         "The v4.0.0-beta.4 release is now recommended.\nhttps://github.com/anza-xyz/agave/releases/tag/v4.0.0-beta.4",
		Permalink:    "https://discord.com/channels/1/2/3",
	}, "card")

	if !strings.Contains(msg, `<b>New</b> 🟢 <b>[solana-tds]</b>`) {
		t.Fatalf("expected card header, got: %s", msg)
	}
	if !strings.Contains(msg, `<i>(Release)</i>`) {
		t.Fatalf("expected release category in header, got: %s", msg)
	}
	if !strings.Contains(msg, `<b>Time:</b> 2026-03-24 06:39 UTC`) {
		t.Fatalf("expected readable UTC timestamp, got: %s", msg)
	}
	if !strings.Contains(msg, `<b>Source:</b> <a href="https://discord.com/channels/1/2/3">Discord</a>`) {
		t.Fatalf("expected clickable short source link, got: %s", msg)
	}
	if strings.Contains(msg, "Source: https://discord.com/channels/1/2/3") {
		t.Fatalf("did not expect raw discord url in source block: %s", msg)
	}
	if strings.Contains(msg, "<code>https://github.com/anza-xyz/agave/releases/tag/v4.0.0-beta.4</code>") {
		t.Fatalf("url should not be wrapped in code tags: %s", msg)
	}
	if !strings.Contains(msg, `<a href="https://github.com/anza-xyz/agave/releases/tag/v4.0.0-beta.4">https://github.com/anza-xyz/agave/releases/tag/v4.0.0-beta.4</a>`) {
		t.Fatalf("expected body link to remain clickable: %s", msg)
	}
}

func TestFormatForwardMessageDetectsActivationCategory(t *testing.T) {
	msg := FormatForwardMessage(model.ForwardRequest{
		SourcePrefix: "[solana-mb]",
		Theme:        "blue",
		AuthorName:   "Solana Tech",
		TimestampISO: "2026-03-23T11:54:45.499Z",
		Text:         "We are going to activate the next feature at the epoch 946. The activation will raise the version floor.",
	}, "card")

	if !strings.Contains(msg, `<i>(Activation)</i>`) {
		t.Fatalf("expected activation category in header, got: %s", msg)
	}
	if !strings.Contains(msg, `🔵 [solana-mb]`) {
		t.Fatalf("expected themed title in header, got: %s", msg)
	}
}

func TestFormatForwardMessageMinimalIsCompact(t *testing.T) {
	msg := FormatForwardMessage(model.ForwardRequest{
		SourcePrefix: "[solana-mb]",
		AuthorName:   "Solana Tech",
		TimestampISO: "2026-03-23T10:04:00.262Z",
		Text:         "Release v3.1.11 is now recommended.",
		Permalink:    "https://discord.com/channels/1/2/4",
		AttachmentURLs: []string{
			"https://example.com/file.txt",
		},
	}, "minimal")

	if strings.Contains(msg, "<b>Author:</b>") {
		t.Fatalf("minimal format should not use card metadata labels: %s", msg)
	}
	if !strings.Contains(msg, "By Solana Tech • 2026-03-23 10:04 UTC") {
		t.Fatalf("expected compact metadata line, got: %s", msg)
	}
	if !strings.Contains(msg, `<b>Files:</b> <a href="https://example.com/file.txt">File 1</a>`) {
		t.Fatalf("expected compact attachment section, got: %s", msg)
	}
}

func TestFormatEditNoticeUsesSummaryInsteadOfLegacyText(t *testing.T) {
	msg := FormatEditNotice(model.EditNoticeRequest{
		SourcePrefix: "[solana-mb]",
		Theme:        "blue",
		TitleBold:    true,
		ChannelName:  "solana-mb",
		Permalink:    "https://discord.com/channels/1/2/5",
		TimestampISO: "2026-03-23T11:54:45.499Z",
		Text:         "@Mainnet Beta Validator\nWe are going to activate the next feature at epoch 946.",
	})

	if strings.Contains(msg, "announcement was edited") {
		t.Fatalf("legacy edit text should not be present: %s", msg)
	}
	if !strings.Contains(msg, "<b>Updated</b> 🔵 <b>[solana-mb]</b>") {
		t.Fatalf("expected updated header, got: %s", msg)
	}
	if !strings.Contains(msg, "activation details changed") {
		t.Fatalf("expected edit reason in header, got: %s", msg)
	}
	if !strings.Contains(msg, `<b>Source:</b> <a href="https://discord.com/channels/1/2/5">Discord</a>`) {
		t.Fatalf("expected clickable source link, got: %s", msg)
	}
	if !strings.Contains(msg, "<b>Context:</b>") {
		t.Fatalf("expected context block, got: %s", msg)
	}
}

func TestFormatInlineTextHighlightsBracketedAndCodeText(t *testing.T) {
	formatted := formatInlineText("[solana-mb] recommended version is `mainnet-v1.44.3`")

	if !strings.Contains(formatted, "<b>[solana-mb]</b>") {
		t.Fatalf("expected bracketed text to be bold, got: %s", formatted)
	}
	if !strings.Contains(formatted, "<pre><code>mainnet-v1.44.3</code></pre>") {
		t.Fatalf("expected single backticks to render as code block, got: %s", formatted)
	}
}

func TestFormatInlineTextRendersTripleBackticksAsCodeBlock(t *testing.T) {
	formatted := formatInlineText("Run:\n```sudo apt update && sudo apt install --only-upgrade doublezero```")

	if !strings.Contains(formatted, "<pre><code>sudo apt update &amp;&amp; sudo apt install --only-upgrade doublezero</code></pre>") {
		t.Fatalf("expected fenced block to render as code block, got: %s", formatted)
	}
}
