package tests

import (
	"fmt"
	"testing"

	"announcementsbot/internal/store"
)

func TestRetentionPrunesOldestAndCascades(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	for i := 1; i <= 4; i++ {
		messageID := fmt.Sprintf("m-%d", i)
		_, err = st.UpsertSeenMessage(messageID, "c1", fmt.Sprintf("h-%d", i), true)
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		err = st.MarkForwarded(store.PersistForwardInput{MessageID: messageID, ChannelID: "c1", ChannelName: "mainnet-anns", TelegramChatID: "tg", TelegramMsgID: fmt.Sprintf("tg-%d", i), MatchedKeywords: []string{"release"}})
		if err != nil {
			t.Fatalf("mark forwarded: %v", err)
		}
		err = st.RecordEditNotice(messageID, fmt.Sprintf("edit-%d", i), fmt.Sprintf("notice-%d", i))
		if err != nil {
			t.Fatalf("record edit: %v", err)
		}
	}

	deleted, err := st.PruneForwardedAnnouncementsToMax(3)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected deleted=1, got %d", deleted)
	}

	counts, err := st.Counts()
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.ForwardedAnnouncements != 3 || counts.ProcessedMessages != 3 || counts.Maps != 3 || counts.Edits != 3 {
		t.Fatalf("unexpected counts: %+v", counts)
	}

	has, err := st.HasForwarded("m-1")
	if err != nil {
		t.Fatalf("has forwarded: %v", err)
	}
	if has {
		t.Fatalf("expected m-1 to be pruned")
	}
}
