package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[37m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
)

type LogEntry struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Data      any    `json:"details,omitempty"`
}

var verbose bool
var prettyLogs = true

func SetVerbose(v bool) {
	verbose = v
}

func SetPrettyLogs(pretty bool) {
	prettyLogs = pretty
}

func getLevelColor(level string) string {
	switch level {
	case "DEBUG":
		return ColorGray
	case "INFO":
		return ColorBlue
	case "WARN":
		return ColorYellow
	case "ERROR":
		return ColorRed
	case "FATAL":
		return ColorRed + ColorBold
	default:
		return ColorReset
	}
}

func formatPrettyLog(level, message string, data any) string {
	timestamp := time.Now().Format("15:04:05")
	levelColor := getLevelColor(level)

	// Format level with fixed width
	levelFormatted := fmt.Sprintf("%-5s", level)

	var output strings.Builder

	// Timestamp in dim gray
	output.WriteString(fmt.Sprintf("%s%s%s ", ColorDim, timestamp, ColorReset))

	// Colored level
	output.WriteString(fmt.Sprintf("%s%s%s ", levelColor, levelFormatted, ColorReset))

	// Message
	output.WriteString(message)

	// Data if present
	if data != nil {
		dataStr := ""
		if dataBytes, err := json.Marshal(data); err == nil {
			dataStr = string(dataBytes)
		} else {
			dataStr = fmt.Sprintf("%+v", data)
		}
		output.WriteString(fmt.Sprintf(" %s%s%s", ColorDim, dataStr, ColorReset))
	}

	return output.String()
}

func log(level, message string, data interface{}) {
	if level == "DEBUG" && !verbose {
		return
	}

	if prettyLogs {
		// Pretty formatted output
		prettyOutput := formatPrettyLog(level, message, data)
		fmt.Fprintln(os.Stdout, prettyOutput)
	} else {
		// JSON formatted output
		entry := LogEntry{
			Level:     level,
			Timestamp: time.Now().Format(time.RFC3339),
			Message:   message,
			Data:      data,
		}
		b, _ := json.Marshal(entry)
		fmt.Fprintf(os.Stdout, "%s\n", b)
	}
}

func Debug(message string, data ...any) {
	log("DEBUG", message, FirstOrNil(data))
}
func Info(message string, data ...any) {
	log("INFO", message, FirstOrNil(data))
}
func Warn(message string, data ...any) {
	log("WARN", message, FirstOrNil(data))
}
func Error(message string, data ...any) {
	log("ERROR", message, FirstOrNil(data))
}
func Fatal(message string, data ...any) {
	log("FATAL", message, FirstOrNil(data))
	os.Exit(1)
}

func FirstOrNil(data []any) any {
	if len(data) == 0 {
		return nil
	}
	return data[0]
}
