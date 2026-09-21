package utils

import (
	"encoding/json"
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
