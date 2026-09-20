package utils

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func ParseVideoUrl(videoURLs []string, message string) (string, error) {
	for _, rawURL := range videoURLs {
		if isVideoURL(rawURL, ".mp4") {
			return rawURL, nil
		}
	}

	// Prefer the first playlist returned by the API. The old implementation
	// retained the last match, which made the selected stream depend on response
	// ordering even within a single quality definition.
	for _, rawURL := range videoURLs {
		if isVideoURL(rawURL, ".m3u8") {
			return rawURL, nil
		}
	}

	return "", fmt.Errorf("未找到回放：%s", message)
}

// ParseRecordVideoURL chooses a recording definition deterministically.
// Go deliberately randomizes map iteration order, so callers must never range
// over the definitions map and accept the first entry.
func ParseRecordVideoURL(definitions map[string][]string, message string) (string, error) {
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := qualityScore(keys[i]), qualityScore(keys[j])
		if left != right {
			return left > right
		}
		return strings.ToLower(keys[i]) > strings.ToLower(keys[j])
	})

	for _, key := range keys {
		if videoURL, err := ParseVideoUrl(definitions[key], message); err == nil {
			return videoURL, nil
		}
	}

	return "", fmt.Errorf("未找到回放：%s", message)
}

func isVideoURL(rawURL, extension string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return strings.Contains(strings.ToLower(parsed.Path), extension)
}

var qualityNumberPattern = regexp.MustCompile(`\d+`)

func qualityScore(definition string) int {
	value := strings.ToLower(strings.TrimSpace(definition))

	qualityLabels := []struct {
		labels []string
		score  int
	}{
		{[]string{"origin", "original", "source", "uhd", "4k", "超清"}, 5000},
		{[]string{"2k", "1440"}, 4000},
		{[]string{"fhd", "1080", "fullhd", "蓝光"}, 3000},
		{[]string{"hd", "720", "high", "高清"}, 2000},
		{[]string{"sd", "480", "standard", "medium", "标清"}, 1000},
		{[]string{"low", "360", "smooth", "流畅"}, 500},
	}
	for _, quality := range qualityLabels {
		for _, label := range quality.labels {
			if strings.Contains(value, label) {
				return quality.score
			}
		}
	}

	highest := 0
	for _, number := range qualityNumberPattern.FindAllString(value, -1) {
		parsed, err := strconv.Atoi(number)
		if err == nil && parsed > highest {
			highest = parsed
		}
	}
	return highest
}
