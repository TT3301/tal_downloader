package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

const (
	DiagnosticLogFileName = "diagnostics.jsonl"
	maxDiagnosticBodySize = 256 * 1024
)

var diagnosticLogMu sync.Mutex

var diagnosticSecretPattern = regexp.MustCompile(`(?i)(token|password|passwd|cookie|authorization|secret|auth_key|sign_token|sms_code|verify_code|phone|mobile|email|account|username)\s*[:=]\s*["']?([^\s,;&"']+)`)

// DiagnosticLogPath returns the local-only diagnostic log path.
// It prefers Fyne's application storage directory and has a safe fallback for
// tests or calls made before the Fyne application is initialized.
func DiagnosticLogPath() string {
	if currentApp := fyne.CurrentApp(); currentApp != nil {
		if storage := currentApp.Storage(); storage != nil {
			if rootURI := storage.RootURI(); rootURI != nil && rootURI.Path() != "" {
				return filepath.Join(rootURI.Path(), DiagnosticLogFileName)
			}
		}
	}

	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "tal_downloader", DiagnosticLogFileName)
	}
	return filepath.Join(os.TempDir(), "tal_downloader", DiagnosticLogFileName)
}

// LogDiagnostic appends one JSON object per line. Logging failures are
// intentionally ignored so diagnostics can never break downloading.
func LogDiagnostic(event string, fields map[string]interface{}) {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"event":     event,
	}
	for key, value := range fields {
		entry[key] = value
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	path := DiagnosticLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}

	diagnosticLogMu.Lock()
	defer diagnosticLogMu.Unlock()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	data = append(data, '\n')
	_, _ = file.Write(data)
}

// ExportDiagnosticLog writes a consistent snapshot of the diagnostic log.
// The same mutex is shared with LogDiagnostic, so an exported JSONL file can
// never end with a partially written event.
func ExportDiagnosticLog(writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("导出目标不可用")
	}

	diagnosticLogMu.Lock()
	defer diagnosticLogMu.Unlock()

	file, err := os.Open(DiagnosticLogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("暂无诊断日志，请先登录并执行一次操作")
		}
		return fmt.Errorf("打开诊断日志失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("写入导出文件失败: %w", err)
	}
	return nil
}

// LogHTTPResponse records API response metadata and a bounded, sanitized body.
// Request headers are deliberately not accepted here, preventing credentials
// from entering the diagnostic log by accident.
func LogHTTPResponse(method, rawURL string, statusCode int, duration time.Duration, body []byte) {
	LogDiagnostic("http_response", map[string]interface{}{
		"method":          method,
		"url":             SanitizeDiagnosticURL(rawURL),
		"status":          statusCode,
		"duration_ms":     duration.Milliseconds(),
		"response_bytes":  len(body),
		"response_sha256": HashDiagnosticBytes(body),
		"response_body":   SanitizeDiagnosticBody(body),
	})
}

func HashDiagnosticBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// SanitizeDiagnosticURL keeps the path and non-sensitive query keys while
// replacing identifiers and signed URL parameters with a fixed marker.
func SanitizeDiagnosticURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid-url]"
	}

	query := parsed.Query()
	for key := range query {
		if sensitiveDiagnosticField(key) || sensitiveURLQueryField(key) {
			query.Set(key, "[REDACTED]")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// SanitizeDiagnosticBody preserves JSON structure and useful course metadata,
// while redacting credentials and signed media URL parameters. Large bodies
// are represented by size and hash instead of being written in full.
func SanitizeDiagnosticBody(body []byte) interface{} {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > maxDiagnosticBodySize {
		return map[string]interface{}{
			"truncated": true,
			"bytes":     len(trimmed),
			"sha256":    HashDiagnosticBytes(trimmed),
		}
	}

	var value interface{}
	if err := json.Unmarshal(trimmed, &value); err == nil {
		return sanitizeDiagnosticValue(value)
	}

	return map[string]interface{}{
		"format": "non-json",
		"text":   sanitizeDiagnosticText(string(trimmed)),
	}
}

func sanitizeDiagnosticValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			if sensitiveDiagnosticField(key) {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = sanitizeDiagnosticValue(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index, item := range typed {
			result[index] = sanitizeDiagnosticValue(item)
		}
		return result
	case string:
		return sanitizeDiagnosticText(typed)
	default:
		return value
	}
}

func sanitizeDiagnosticText(value string) string {
	parsed, err := url.Parse(value)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return SanitizeDiagnosticURL(value)
	}
	return diagnosticSecretPattern.ReplaceAllString(value, "$1=[REDACTED]")
}

func sensitiveDiagnosticField(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "passwd") ||
		strings.Contains(lower, "cookie") ||
		strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "signature") ||
		strings.Contains(lower, "sign_token") ||
		strings.Contains(lower, "auth_key") ||
		strings.Contains(lower, "verify_code") ||
		strings.Contains(lower, "sms_code") ||
		strings.Contains(lower, "phone") ||
		strings.Contains(lower, "mobile") ||
		strings.Contains(lower, "email") ||
		lower == "code" ||
		lower == "account" ||
		lower == "username" ||
		lower == "stuid" ||
		lower == "user_id" ||
		lower == "userid" ||
		lower == "pu_uid"
}

func sensitiveURLQueryField(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return lower == "auth_key" || lower == "expires" || lower == "expire" ||
		lower == "timestamp" || lower == "sign" || lower == "sig" ||
		lower == "key"
}
