package utils

import "testing"

func TestParseRecordVideoURLUsesDeterministicHighestQuality(t *testing.T) {
	definitions := map[string][]string{
		"标清": {"https://media.example/lesson-sd.mp4?token=1"},
		"高清": {"https://media.example/lesson-hd.m3u8?token=2"},
		"1080P": {
			"not-a-url",
			"https://media.example/lesson-full.mp4?token=3",
		},
	}

	for i := 0; i < 100; i++ {
		got, err := ParseRecordVideoURL(definitions, "")
		if err != nil {
			t.Fatalf("ParseRecordVideoURL: %v", err)
		}
		if want := "https://media.example/lesson-full.mp4?token=3"; got != want {
			t.Fatalf("iteration %d: got %q, want %q", i, got, want)
		}
	}
}

func TestParseVideoURLRejectsNonMediaResponse(t *testing.T) {
	_, err := ParseVideoUrl([]string{
		"https://media.example/error.json?file=.mp4",
		"javascript:alert(1).m3u8",
	}, "not ready")
	if err == nil {
		t.Fatal("expected an error for non-media URLs")
	}
}
