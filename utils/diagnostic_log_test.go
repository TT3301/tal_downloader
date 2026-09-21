package utils

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeDiagnosticURL(t *testing.T) {
	input := "https://example.test/course?stuId=student-1&page=1&token=secret&foo=bar"
	got := SanitizeDiagnosticURL(input)

	if strings.Contains(got, "student-1") || strings.Contains(got, "secret") {
		t.Fatalf("sensitive URL value leaked: %s", got)
	}
	if !strings.Contains(got, "page=1") || !strings.Contains(got, "foo=bar") {
		t.Fatalf("non-sensitive URL values were not preserved: %s", got)
	}
}

func TestSanitizeDiagnosticBodyPreservesCourseDataAndRedactsSecrets(t *testing.T) {
	input := []byte(`{
		"stdCourseId":"course-1",
		"courseName":"【科学思维】初一暑期JS",
		"token":"secret-token",
		"videoUrls":["https://media.example/video.m3u8?auth_key=secret-key&quality=hd"]
	}`)

	sanitized, err := json.Marshal(SanitizeDiagnosticBody(input))
	if err != nil {
		t.Fatal(err)
	}
	got := string(sanitized)
	if !strings.Contains(got, "course-1") || !strings.Contains(got, "初一暑期JS") {
		t.Fatalf("course data was lost: %s", got)
	}
	if strings.Contains(got, "secret-token") || strings.Contains(got, "secret-key") {
		t.Fatalf("secret leaked: %s", got)
	}
	if !strings.Contains(got, "quality=hd") || !strings.Contains(got, "REDACTED") {
		t.Fatalf("signed URL was not sanitized as expected: %s", got)
	}
}

func TestSanitizeDiagnosticBodyRedactsPlainTextSecrets(t *testing.T) {
	input := []byte(`request failed: token=secret-token password:"secret-pass" phone=13800138000`)

	sanitized, err := json.Marshal(SanitizeDiagnosticBody(input))
	if err != nil {
		t.Fatal(err)
	}
	got := string(sanitized)
	for _, secret := range []string{"secret-token", "secret-pass", "13800138000"} {
		if strings.Contains(got, secret) {
			t.Fatalf("plain text secret leaked: %s", got)
		}
	}
}

func TestSanitizeDiagnosticBodyRedactsAdditionalJSONFields(t *testing.T) {
	input := []byte(`{"courseName":"保留课程名","auth_key":"secret-key","mobile":"13800138000","email":"user@example.test"}`)

	sanitized, err := json.Marshal(SanitizeDiagnosticBody(input))
	if err != nil {
		t.Fatal(err)
	}
	got := string(sanitized)
	if !strings.Contains(got, "保留课程名") {
		t.Fatalf("useful course data was lost: %s", got)
	}
	for _, secret := range []string{"secret-key", "13800138000", "user@example.test"} {
		if strings.Contains(got, secret) {
			t.Fatalf("JSON secret leaked: %s", got)
		}
	}
}

func TestExportDiagnosticLog(t *testing.T) {
	path := DiagnosticLogPath()
	original, readErr := os.ReadFile(path)
	hadOriginal := readErr == nil
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.MkdirAll(filepath.Dir(path), 0755)
			_ = os.WriteFile(path, original, 0600)
		} else {
			_ = os.Remove(path)
		}
	})

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	want := []byte("{\"event\":\"one\"}\n{\"event\":\"two\"}\n")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := ExportDiagnosticLog(&output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("export mismatch: got %q, want %q", output.Bytes(), want)
	}
}
