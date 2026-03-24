package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"announcementsbot/internal/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		payload, _ := json.Marshal(map[string]any{
			"ts":    time.Now().UTC().Format(time.RFC3339Nano),
			"level": "error",
			"msg":   "Fatal startup error",
			"error": err.Error(),
		})
		fmt.Fprintln(os.Stderr, string(payload))
		os.Exit(1)
	}
}
