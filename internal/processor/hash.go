package processor

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"announcementsbot/internal/model"
)

func contentHash(event *model.AnnouncementEvent) string {
	payload := event.ContentText + "\n" + event.EmbedText + "\n" + strings.Join(event.AttachmentURLs, ",")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
