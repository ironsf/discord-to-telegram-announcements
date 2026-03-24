package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Logger struct {
	level int
}

const (
	levelDebug = 10
	levelInfo  = 20
	levelWarn  = 30
	levelError = 40
)

func New(level string) *Logger {
	switch strings.ToLower(level) {
	case "debug":
		return &Logger{level: levelDebug}
	case "warn":
		return &Logger{level: levelWarn}
	case "error":
		return &Logger{level: levelError}
	default:
		return &Logger{level: levelInfo}
	}
}

func (l *Logger) Debug(msg string, meta map[string]any) { l.write(levelDebug, "debug", msg, meta) }
func (l *Logger) Info(msg string, meta map[string]any)  { l.write(levelInfo, "info", msg, meta) }
func (l *Logger) Warn(msg string, meta map[string]any)  { l.write(levelWarn, "warn", msg, meta) }
func (l *Logger) Error(msg string, meta map[string]any) { l.write(levelError, "error", msg, meta) }

func (l *Logger) write(level int, name, msg string, meta map[string]any) {
	if level < l.level {
		return
	}
	record := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": name,
		"msg":   msg,
	}
	for k, v := range meta {
		record[k] = v
	}
	line, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintf(os.Stderr, "{\"ts\":\"%s\",\"level\":\"error\",\"msg\":\"logger marshal failed\",\"error\":%q}\n", time.Now().UTC().Format(time.RFC3339Nano), err.Error())
		return
	}
	if level >= levelWarn {
		fmt.Fprintln(os.Stderr, string(line))
		return
	}
	fmt.Println(string(line))
}
