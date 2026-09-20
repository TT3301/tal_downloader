package downloader

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itsHenry35/tal_downloader/utils"
)

func (d *Downloader) downloadM3U8(task *DownloadTask) error {
	task.SetStatus("preparing")
	task.StartTime = time.Now()
	atomic.StoreInt64(&task.Downloaded, 0)
	atomic.StoreInt64(&task.DownloadedParts, 0)
	task.clearError()

	// 创建临时目录
	tmpDir := filepath.Join(filepath.Dir(task.FilePath), fmt.Sprintf(".tmp_%d_%s", time.Now().UnixNano(), filepath.Base(task.FilePath)))

	// 使用安卓安全的目录创建
	if err := utils.Mkdir(tmpDir); err != nil {
		return err
	}
	defer func() {
		if utils.IsAndroid() {
			_ = os.RemoveAll(utils.GetAndroidSafeFilePath(tmpDir))
		} else {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	segmentURLs, err := loadHLSMediaPlaylist(d.client, task.URL, 0)
	if err != nil {
		return err
	}
	if len(segmentURLs) == 0 {
		return fmt.Errorf("M3U8 播放列表中没有媒体分片")
	}
	task.SetStatus("downloading")

	// 使用负数存储总段数，便于进度管理器识别M3U8任务
	task.TotalSize = -int64(len(segmentURLs))

	// 将任务添加到进度管理器
	d.progressManager.AddTask(task)
	defer d.progressManager.RemoveTask(task)

	var wg sync.WaitGroup
	concurrency := d.perFileThreads
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)

	errorChannel := make(chan error, len(segmentURLs))
	for idx, segmentURL := range segmentURLs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rawURL string) {
			defer wg.Done()
			defer func() { <-sem }()

			filePath := filepath.Join(tmpDir, fmt.Sprintf("%05d.ts", i))
			var lastError error
			for attempt := 0; attempt < 3; attempt++ {
				lastError = downloadTS(d.client, rawURL, filePath, task)
				if lastError == nil {
					atomic.AddInt64(&task.DownloadedParts, 1)
					return
				}
				if attempt < 2 {
					time.Sleep(time.Second)
				}
			}
			errorChannel <- fmt.Errorf("分片 %d 下载失败：%w", i+1, lastError)
		}(idx, segmentURL)
	}

	wg.Wait()
	close(errorChannel)
	if err := <-errorChannel; err != nil {
		return err
	}

	// 进入合并阶段，进度管理器会自动显示90%进度
	task.SetStatus("merging")

	// 合并TS文件
	err = mergeTSFiles(tmpDir, task.FilePath, len(segmentURLs))

	if err == nil {
		task.SetStatus("completed")
		// 手动发送最终完成进度
		if task.progress != nil {
			// 获取最终文件大小
			actualOutputPath := task.FilePath
			if utils.IsAndroid() {
				actualOutputPath = utils.GetAndroidSafeFilePath(task.FilePath)
			}

			stat, statErr := os.Stat(actualOutputPath)
			if statErr == nil {
				task.progress(100, "Completed", -1, stat.Size())
			} else {
				task.progress(100, "Completed", -1, atomic.LoadInt64(&task.Downloaded))
			}
		}
	}

	return err
}

func downloadTS(client *http.Client, url, filePath string, task *DownloadTask) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := validateMediaResponse(resp); err != nil {
		return err
	}

	out, err := utils.CreateFile(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 使用缓冲读取以支持暂停功能
	buf := make([]byte, 32*1024)
	var totalBytes int64

	for {
		// 检查暂停状态
		if task.isPaused.Load() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			totalBytes += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	atomic.AddInt64(&task.Downloaded, totalBytes)
	return nil
}

func mergeTSFiles(tmpDir, outputFile string, expectedFiles int) error {
	out, err := utils.CreateFile(outputFile)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = out.Close()
		if !succeeded {
			_ = utils.RemoveFile(outputFile)
		}
	}()

	// 获取实际的临时目录路径
	actualTmpDir := tmpDir
	if utils.IsAndroid() {
		actualTmpDir = utils.GetAndroidSafeFilePath(tmpDir)
	}

	files, err := os.ReadDir(actualTmpDir)
	if err != nil {
		return err
	}

	// 按名字排序，确保顺序
	var tsFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".ts") {
			tsFiles = append(tsFiles, f.Name())
		}
	}
	sort.Strings(tsFiles)
	if len(tsFiles) != expectedFiles {
		return fmt.Errorf("媒体分片不完整：找到 %d/%d 个", len(tsFiles), expectedFiles)
	}

	for _, name := range tsFiles {
		path := filepath.Join(actualTmpDir, name)
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	if err := out.Sync(); err != nil {
		return err
	}
	succeeded = true
	return nil
}

type hlsVariant struct {
	url       string
	bandwidth int64
}

var (
	hlsBandwidthPattern = regexp.MustCompile(`(?i)(?:^|,)\s*BANDWIDTH=(\d+)`)
	hlsURIAttribute     = regexp.MustCompile(`(?i)(?:^|,)\s*URI="([^"]+)"`)
)

func loadHLSMediaPlaylist(client *http.Client, rawURL string, depth int) ([]string, error) {
	if depth > 4 {
		return nil, fmt.Errorf("M3U8 播放列表嵌套过深")
	}

	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("M3U8 请求失败：HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(body), "#EXTM3U") {
		return nil, fmt.Errorf("服务器返回的不是有效 M3U8 播放列表")
	}

	lines := strings.Split(string(body), "\n")
	segments := make([]string, 0)
	variants := make([]hlsVariant, 0)
	pendingBandwidth := int64(-1)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "#EXT-X-KEY:") && !strings.Contains(upperLine, "METHOD=NONE") {
			return nil, fmt.Errorf("暂不支持加密的 HLS 回放")
		}
		if strings.HasPrefix(upperLine, "#EXT-X-MAP:") {
			match := hlsURIAttribute.FindStringSubmatch(line[strings.Index(line, ":")+1:])
			if len(match) == 2 {
				resolved, err := resolveHLSURL(rawURL, match[1])
				if err != nil {
					return nil, err
				}
				segments = append(segments, resolved)
			}
			continue
		}
		if strings.HasPrefix(upperLine, "#EXT-X-STREAM-INF:") {
			pendingBandwidth = 0
			if match := hlsBandwidthPattern.FindStringSubmatch(line[strings.Index(line, ":")+1:]); len(match) == 2 {
				pendingBandwidth, _ = strconv.ParseInt(match[1], 10, 64)
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		resolved, err := resolveHLSURL(rawURL, line)
		if err != nil {
			return nil, err
		}
		if pendingBandwidth >= 0 {
			variants = append(variants, hlsVariant{url: resolved, bandwidth: pendingBandwidth})
			pendingBandwidth = -1
			continue
		}
		segments = append(segments, resolved)
	}

	if len(variants) > 0 {
		sort.SliceStable(variants, func(i, j int) bool {
			if variants[i].bandwidth != variants[j].bandwidth {
				return variants[i].bandwidth > variants[j].bandwidth
			}
			return variants[i].url < variants[j].url
		})
		return loadHLSMediaPlaylist(client, variants[0].url, depth+1)
	}

	if len(segments) == 1 && strings.Contains(strings.ToLower(segments[0]), ".m3u8") {
		return loadHLSMediaPlaylist(client, segments[0], depth+1)
	}
	return segments, nil
}

func resolveHLSURL(baseURL, reference string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(reference)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	baseQuery := base.Query()
	resolvedQuery := resolved.Query()
	for key, values := range baseQuery {
		if _, exists := resolvedQuery[key]; exists {
			continue
		}
		for _, value := range values {
			resolvedQuery.Add(key, value)
		}
	}
	resolved.RawQuery = resolvedQuery.Encode()
	return resolved.String(), nil
}
