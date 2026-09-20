package downloader

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadRegularFileWithVerifiedRanges(t *testing.T) {
	content := bytes.Repeat([]byte("0123456789abcdef"), 64)
	server := newRangeServer(t, content, false)
	defer server.Close()

	downloader := NewDownloader(1, 8)
	downloader.client = server.Client()
	filePath := filepath.Join(t.TempDir(), "lesson.mp4")
	task := &DownloadTask{URL: server.URL, FilePath: filePath}

	if err := downloader.downloadRegularFile(task); err != nil {
		t.Fatalf("downloadRegularFile: %v", err)
	}
	assertFileContent(t, filePath, content)
}

func TestDownloadRegularFileFallsBackWhenRangeIsIgnored(t *testing.T) {
	content := []byte("server returned the complete video")
	server := newRangeServer(t, content, true)
	defer server.Close()

	downloader := NewDownloader(1, 8)
	downloader.client = server.Client()
	filePath := filepath.Join(t.TempDir(), "lesson.mp4")
	task := &DownloadTask{URL: server.URL, FilePath: filePath}

	if err := downloader.downloadRegularFile(task); err != nil {
		t.Fatalf("downloadRegularFile: %v", err)
	}
	assertFileContent(t, filePath, content)
}

func TestDownloadRegularFileRejectsBrokenRangeAndRemovesPartialFile(t *testing.T) {
	content := bytes.Repeat([]byte("video"), 128)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNumber := requests.Add(1)
		if requestNumber == 1 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(content)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[:1])
			return
		}
		// A CDN that ignores later Range requests used to corrupt every chunk.
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	downloader := NewDownloader(1, 4)
	downloader.client = server.Client()
	filePath := filepath.Join(t.TempDir(), "lesson.mp4")
	task := &DownloadTask{URL: server.URL, FilePath: filePath}

	if err := downloader.downloadRegularFile(task); err == nil {
		t.Fatal("expected a range validation error")
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("partial file should be removed, stat error = %v", err)
	}
}

func TestStartConsumesPendingQueueOnlyOnce(t *testing.T) {
	content := []byte("one queued download")
	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, _ = w.Write(content)
	}))
	defer server.Close()

	downloader := NewDownloader(1, 2)
	downloader.client = server.Client()
	task := downloader.AddTask(server.URL, filepath.Join(t.TempDir(), "lesson.mp4"), nil)
	downloader.Start()
	task.Wait()
	requestsAfterFirstStart := requestCount.Load()
	if requestsAfterFirstStart != 2 {
		t.Fatalf("first start made %d requests, want probe + download", requestsAfterFirstStart)
	}

	downloader.Start()
	if got := requestCount.Load(); got != requestsAfterFirstStart {
		t.Fatalf("old task ran again: requests = %d, want %d", got, requestsAfterFirstStart)
	}
}

func TestRetryRestartsFailedTask(t *testing.T) {
	content := []byte("retry completed video")
	var fullRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}
		if fullRequests.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, _ = w.Write(content)
	}))
	defer server.Close()

	downloader := NewDownloader(1, 2)
	downloader.client = server.Client()
	filePath := filepath.Join(t.TempDir(), "lesson.mp4")
	task := downloader.AddTask(server.URL, filePath, nil)
	downloader.Start()
	task.Wait()
	if got, want := task.Status(), "error"; got != want {
		t.Fatalf("first status = %q, want %q", got, want)
	}
	if err := downloader.Retry(task); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	task.Wait()
	if got, want := task.Status(), "completed"; got != want {
		t.Fatalf("retry status = %q, want %q", got, want)
	}
	assertFileContent(t, filePath, content)
	if err := downloader.Retry(task); err == nil {
		t.Fatal("completed task should not be retried")
	}
}

func TestLoadHLSMediaPlaylistSelectsHighestVariantAndResolvesURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100\nlow/list.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=900\nhigh/list.m3u8\n")
		case "/high/list.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXTINF:1,\n../segments/one.ts\n#EXTINF:1,\ntwo.ts?part=2\n")
		case "/low/list.m3u8":
			fmt.Fprint(w, "#EXTM3U\nlow.ts\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	segments, err := loadHLSMediaPlaylist(server.Client(), server.URL+"/master.m3u8?token=abc", 0)
	if err != nil {
		t.Fatalf("loadHLSMediaPlaylist: %v", err)
	}
	want := []string{
		server.URL + "/segments/one.ts?token=abc",
		server.URL + "/high/two.ts?part=2&token=abc",
	}
	if len(segments) != len(want) {
		t.Fatalf("segments = %#v, want %#v", segments, want)
	}
	for i := range want {
		if segments[i] != want[i] {
			t.Fatalf("segment %d = %q, want %q", i, segments[i], want[i])
		}
	}
}

func TestDownloadM3U8FailsWhenAnySegmentIsMissing(t *testing.T) {
	var failedSegmentRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lesson.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXTINF:1,\ngood.ts\n#EXTINF:1,\nmissing.ts\n")
		case "/good.ts":
			w.Header().Set("Content-Type", "video/mp2t")
			_, _ = w.Write([]byte("good-segment"))
		case "/missing.ts":
			failedSegmentRequests.Add(1)
			http.Error(w, "missing", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	downloader := NewDownloader(1, 2)
	downloader.client = server.Client()
	filePath := filepath.Join(t.TempDir(), "lesson.mp4")
	task := &DownloadTask{URL: server.URL + "/lesson.m3u8", FilePath: filePath}

	if err := downloader.downloadM3U8(task); err == nil {
		t.Fatal("expected a missing segment to fail the whole download")
	}
	if got, want := failedSegmentRequests.Load(), int64(3); got != want {
		t.Fatalf("failed segment attempts = %d, want %d", got, want)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("incomplete HLS output should not exist, stat error = %v", err)
	}
}

func TestParseContentRange(t *testing.T) {
	start, end, total, err := parseContentRange("bytes 10-19/100")
	if err != nil {
		t.Fatalf("parseContentRange: %v", err)
	}
	if start != 10 || end != 19 || total != 100 {
		t.Fatalf("got %d-%d/%d", start, end, total)
	}
	if _, _, _, err := parseContentRange("bytes */100"); err == nil {
		t.Fatal("expected malformed range to fail")
	}
}

func newRangeServer(t *testing.T, content []byte, ignoreRange bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		rangeHeader := r.Header.Get("Range")
		if ignoreRange || rangeHeader == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}

		var start, end int
		if _, err := fmt.Sscanf(strings.TrimPrefix(rangeHeader, "bytes="), "%d-%d", &start, &end); err != nil {
			t.Fatalf("invalid test range %q: %v", rangeHeader, err)
		}
		if end >= len(content) {
			end = len(content) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file content mismatch: got %d bytes, want %d", len(got), len(want))
	}
}
