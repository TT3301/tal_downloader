package downloader

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itsHenry35/tal_downloader/utils"
)

type DownloadTask struct {
	URL             string
	FilePath        string
	TotalSize       int64
	Downloaded      int64
	DownloadedParts int64 //used in m3u8 downloads
	StartTime       time.Time
	Error           error

	progress   func(float64, string, int64, int64)
	cancelFunc func()
	isPaused   atomic.Bool
	wg         sync.WaitGroup

	// 进度更新相关
	progressMutex    sync.Mutex
	lastProgressTime time.Time
	lastDownloaded   int64

	// 状态相关
	statusMutex sync.RWMutex
	status      string
	errorMutex  sync.RWMutex
}

// Status getter and setter methods for thread safety
func (task *DownloadTask) Status() string {
	task.statusMutex.RLock()
	defer task.statusMutex.RUnlock()
	return task.status
}

func (task *DownloadTask) SetStatus(status string) {
	task.statusMutex.Lock()
	defer task.statusMutex.Unlock()
	task.status = status
}

func (task *DownloadTask) setError(err error) {
	if err == nil {
		return
	}
	task.errorMutex.Lock()
	defer task.errorMutex.Unlock()
	if task.Error == nil {
		task.Error = err
	}
}

func (task *DownloadTask) getError() error {
	task.errorMutex.RLock()
	defer task.errorMutex.RUnlock()
	return task.Error
}

func (task *DownloadTask) clearError() {
	task.errorMutex.Lock()
	task.Error = nil
	task.errorMutex.Unlock()
}

type ProgressManager struct {
	tasks      map[*DownloadTask]bool
	tasksMutex sync.RWMutex
	stopChan   chan struct{}
	started    bool
	startMutex sync.Mutex // 保护started字段
}

func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		tasks: make(map[*DownloadTask]bool),
	}
}

func (pm *ProgressManager) AddTask(task *DownloadTask) {
	pm.tasksMutex.Lock()
	defer pm.tasksMutex.Unlock()

	task.progressMutex.Lock()
	task.lastProgressTime = time.Now()
	task.lastDownloaded = 0
	task.progressMutex.Unlock()

	pm.tasks[task] = true

	pm.startMutex.Lock()
	shouldStart := !pm.started
	if shouldStart {
		pm.started = true
		pm.stopChan = make(chan struct{})
	}
	stopChan := pm.stopChan
	pm.startMutex.Unlock()

	if shouldStart {
		pm.start(stopChan)
	}
}

func (pm *ProgressManager) RemoveTask(task *DownloadTask) {
	pm.tasksMutex.Lock()
	defer pm.tasksMutex.Unlock()

	delete(pm.tasks, task)

	pm.startMutex.Lock()
	shouldStop := len(pm.tasks) == 0 && pm.started
	if shouldStop {
		pm.started = false
		close(pm.stopChan)
	}
	pm.startMutex.Unlock()
}

func (pm *ProgressManager) start(stopChan <-chan struct{}) {
	ticker := time.NewTicker(100 * time.Millisecond)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pm.updateAllTasks()
			case <-stopChan:
				return
			}
		}
	}()
}

func (pm *ProgressManager) updateAllTasks() {
	pm.tasksMutex.RLock()
	taskList := make([]*DownloadTask, 0, len(pm.tasks))
	for task := range pm.tasks {
		taskList = append(taskList, task)
	}
	pm.tasksMutex.RUnlock()

	now := time.Now()
	for _, task := range taskList {
		if task.progress == nil {
			continue
		}

		status := task.Status()
		if status != "downloading" && status != "merging" {
			continue
		}

		pm.updateSingleTask(task, now, status)
	}
}

func (pm *ProgressManager) updateSingleTask(task *DownloadTask, now time.Time, status string) {
	task.progressMutex.Lock()
	defer task.progressMutex.Unlock()

	if task.TotalSize < 0 {
		// M3U8下载的进度计算（TotalSize为负数存储总段数）
		totalParts := -task.TotalSize

		if status == "merging" {
			// 合并阶段显示90%进度
			if task.progress != nil {
				task.progress(90, "合并中", atomic.LoadInt64(&task.Downloaded), -1)
			}
			return
		}

		// 下载阶段的进度计算
		completed := atomic.LoadInt64(&task.DownloadedParts)

		if task.lastProgressTime.IsZero() {
			task.lastProgressTime = now
			task.lastDownloaded = completed
			return
		}

		timeDiff := now.Sub(task.lastProgressTime).Seconds()
		if timeDiff > 0 {
			speed := float64(completed-task.lastDownloaded) / timeDiff
			percent := float64(completed) / float64(totalParts) * 90 // 最多到90%，留10%给合并
			if task.progress != nil {
				task.progress(percent, fmt.Sprintf("%.2f ts/s (%d/%d)", speed, completed, totalParts), atomic.LoadInt64(&task.Downloaded), -1)
			}
			task.lastProgressTime = now
			task.lastDownloaded = completed
		}
	} else if task.TotalSize > 0 {
		// 普通文件下载的进度计算
		downloaded := atomic.LoadInt64(&task.Downloaded)
		if task.lastProgressTime.IsZero() {
			task.lastProgressTime = now
			task.lastDownloaded = downloaded
			return
		}

		timeDiff := now.Sub(task.lastProgressTime).Seconds()
		if timeDiff > 0 {
			bytes := downloaded - task.lastDownloaded
			speed := float64(bytes) / timeDiff / 1024 / 1024
			progress := float64(downloaded) / float64(task.TotalSize) * 100
			if task.progress != nil {
				task.progress(progress, fmt.Sprintf("%.2f MB/s", speed), downloaded, task.TotalSize)
			}
			task.lastProgressTime = now
			task.lastDownloaded = downloaded
		}
	}
}

type Downloader struct {
	concurrentFiles int
	perFileThreads  int
	tasks           []*DownloadTask
	mu              sync.Mutex
	client          *http.Client
	progressManager *ProgressManager
}

func NewDownloader(concurrentFiles, perFileThreads int) *Downloader {
	transport := &http.Transport{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 512,
		MaxConnsPerHost:     512,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Downloader{
		concurrentFiles: concurrentFiles,
		perFileThreads:  perFileThreads,
		progressManager: NewProgressManager(),
		client: &http.Client{
			Timeout:   0,
			Transport: transport,
		},
	}
}

func (d *Downloader) AddTask(url, filePath string, progressFunc func(float64, string, int64, int64)) *DownloadTask {
	task := &DownloadTask{
		URL:      url,
		FilePath: filePath,
		progress: progressFunc,
	}
	task.SetStatus("pending")
	d.mu.Lock()
	d.tasks = append(d.tasks, task)
	d.mu.Unlock()
	return task
}

func (d *Downloader) Start() {
	d.mu.Lock()
	pendingTasks := append([]*DownloadTask(nil), d.tasks...)
	d.tasks = nil
	d.mu.Unlock()

	concurrentFiles := d.concurrentFiles
	if concurrentFiles < 1 {
		concurrentFiles = 1
	}
	semaphore := make(chan struct{}, concurrentFiles)

	for _, task := range pendingTasks {
		d.startTask(task, semaphore)
	}
}

func (d *Downloader) startTask(task *DownloadTask, semaphore chan struct{}) {
	task.wg.Add(1)
	go func(t *DownloadTask) {
		defer t.wg.Done()
		if semaphore != nil {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
		}
		if err := d.downloadFile(t); err != nil {
			t.setError(err)
			t.SetStatus("error")
			if t.progress != nil {
				t.progress(0, fmt.Sprintf("错误： %v", err), -1, -1)
			}
		}
	}(task)
}

// Retry restarts a failed task. Resume only controls a live paused request and
// cannot revive a goroutine that has already returned with an error.
func (d *Downloader) Retry(task *DownloadTask) error {
	if task == nil {
		return fmt.Errorf("下载任务为空")
	}

	task.statusMutex.Lock()
	if task.status != "error" {
		status := task.status
		task.statusMutex.Unlock()
		return fmt.Errorf("只有失败任务可以重试，当前状态：%s", status)
	}
	task.status = "pending"
	task.statusMutex.Unlock()

	task.clearError()
	task.isPaused.Store(false)
	atomic.StoreInt64(&task.Downloaded, 0)
	atomic.StoreInt64(&task.DownloadedParts, 0)
	d.startTask(task, nil)
	return nil
}

func (d *Downloader) downloadRegularFile(task *DownloadTask) error {
	task.SetStatus("preparing")
	task.StartTime = time.Now()
	atomic.StoreInt64(&task.Downloaded, 0)
	task.clearError()

	// 创建目录
	if err := utils.Mkdir(filepath.Dir(task.FilePath)); err != nil {
		return err
	}

	supportsRange, totalSize, err := d.probeRegularFile(task.URL)
	if err != nil {
		return err
	}
	task.TotalSize = totalSize

	if !supportsRange || totalSize <= 0 || d.perFileThreads <= 1 {
		return d.downloadSingleThread(task)
	}

	return d.downloadMultiThread(task)
}

func (d *Downloader) probeRegularFile(rawURL string) (bool, int64, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err := d.client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if err := validateMediaResponse(resp); err != nil {
		return false, 0, err
	}
	if resp.StatusCode == http.StatusPartialContent {
		start, end, total, err := parseContentRange(resp.Header.Get("Content-Range"))
		if err != nil || start != 0 || end != 0 || total <= 0 {
			return false, 0, fmt.Errorf("服务器返回了无效的 Content-Range: %q", resp.Header.Get("Content-Range"))
		}
		return true, total, nil
	}
	if resp.StatusCode == http.StatusOK {
		total := resp.ContentLength
		if total < 0 {
			total = 0
		}
		return false, total, nil
	}
	return false, 0, fmt.Errorf("视频探测失败：HTTP %d", resp.StatusCode)
}

func (d *Downloader) downloadMultiThread(task *DownloadTask) error {
	file, err := utils.CreateFile(task.FilePath)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = file.Close()
		if !succeeded {
			_ = utils.RemoveFile(task.FilePath)
		}
	}()

	// 预分配文件大小
	if err := file.Truncate(task.TotalSize); err != nil {
		return err
	}

	threadCount := d.perFileThreads
	if int64(threadCount) > task.TotalSize {
		threadCount = int(task.TotalSize)
	}
	if threadCount < 1 {
		return fmt.Errorf("视频大小无效：%d", task.TotalSize)
	}
	partSize := task.TotalSize / int64(threadCount)
	var wg sync.WaitGroup

	task.SetStatus("downloading")

	// 将任务添加到进度管理器
	d.progressManager.AddTask(task)
	defer d.progressManager.RemoveTask(task)

	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		start := int64(i) * partSize
		end := start + partSize - 1
		if i == threadCount-1 {
			end = task.TotalSize - 1
		}

		go func(start, end int64) {
			defer wg.Done()
			if task.getError() != nil {
				return
			}
			req, err := http.NewRequest("GET", task.URL, nil)
			if err != nil {
				task.setError(err)
				return
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

			resp, err := d.client.Do(req)
			if err != nil {
				task.setError(err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusPartialContent {
				task.setError(fmt.Errorf("服务器未按 Range 返回分片：HTTP %d", resp.StatusCode))
				return
			}
			responseStart, responseEnd, total, err := parseContentRange(resp.Header.Get("Content-Range"))
			if err != nil || responseStart != start || responseEnd != end || total != task.TotalSize {
				task.setError(fmt.Errorf("分片范围不匹配：请求 %d-%d，返回 %q", start, end,
					resp.Header.Get("Content-Range")))
				return
			}
			if err := validateMediaResponse(resp); err != nil {
				task.setError(err)
				return
			}

			buf := make([]byte, 32*1024)
			offset := start
			for {
				if task.getError() != nil {
					return
				}
				if task.isPaused.Load() {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				n, err := resp.Body.Read(buf)
				if n > 0 {
					if offset+int64(n) > end+1 {
						task.setError(fmt.Errorf("分片 %d-%d 返回数据超出范围", start, end))
						return
					}
					_, err = file.WriteAt(buf[:n], offset)
					if err != nil {
						task.setError(err)
						return
					}
					offset += int64(n)
					atomic.AddInt64(&task.Downloaded, int64(n))
				}
				if err == io.EOF {
					if offset != end+1 {
						task.setError(fmt.Errorf("分片 %d-%d 不完整：收到 %d 字节", start, end, offset-start))
					}
					break
				}
				if err != nil {
					task.setError(err)
					return
				}
			}
		}(start, end)
	}

	wg.Wait()

	if task.getError() == nil && atomic.LoadInt64(&task.Downloaded) == task.TotalSize {
		succeeded = true
		task.SetStatus("completed")
		if task.progress != nil {
			task.progress(100, "Completed", task.TotalSize, task.TotalSize)
		}
	} else if task.getError() == nil {
		task.setError(fmt.Errorf("视频文件不完整：收到 %d/%d 字节", atomic.LoadInt64(&task.Downloaded), task.TotalSize))
	}
	return task.getError()
}

func (d *Downloader) downloadSingleThread(task *DownloadTask) error {
	resp, err := d.client.Get(task.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := validateMediaResponse(resp); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("视频下载失败：HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > 0 {
		task.TotalSize = resp.ContentLength
	}

	file, err := utils.CreateFile(task.FilePath)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = file.Close()
		if !succeeded {
			_ = utils.RemoveFile(task.FilePath)
		}
	}()

	task.SetStatus("downloading")
	task.StartTime = time.Now()

	// 将任务添加到进度管理器
	d.progressManager.AddTask(task)
	defer d.progressManager.RemoveTask(task)

	buf := make([]byte, 32*1024)

	for {
		if task.isPaused.Load() {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, err := file.Write(buf[:n])
			if err != nil {
				return err
			}
			atomic.AddInt64(&task.Downloaded, int64(n))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if task.TotalSize > 0 && atomic.LoadInt64(&task.Downloaded) != task.TotalSize {
		return fmt.Errorf("视频文件不完整：收到 %d/%d 字节", atomic.LoadInt64(&task.Downloaded), task.TotalSize)
	}

	succeeded = true
	task.SetStatus("completed")
	if task.progress != nil {
		task.progress(100, "Completed", atomic.LoadInt64(&task.Downloaded), task.TotalSize)
	}
	return nil
}

func parseContentRange(value string) (int64, int64, int64, error) {
	var start, end, total int64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, 0, 0, err
	}
	if start < 0 || end < start || total <= end {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range")
	}
	return start, end, total, nil
}

func validateMediaResponse(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("媒体服务器返回 HTTP %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/json") {
		return fmt.Errorf("媒体服务器返回了非视频内容（%s）", contentType)
	}
	return nil
}

func (task *DownloadTask) Pause() {
	task.isPaused.Store(true)
}

func (task *DownloadTask) Resume() {
	task.isPaused.Store(false)
}

func (task *DownloadTask) IsPaused() bool {
	return task.isPaused.Load()
}

func (task *DownloadTask) Cancel() {
	if task.cancelFunc != nil {
		task.cancelFunc()
	}
	task.SetStatus("cancelled")
}

func (task *DownloadTask) Wait() {
	task.wg.Wait()
}

func (d *Downloader) downloadFile(task *DownloadTask) error {
	if strings.Contains(strings.ToLower(task.URL), ".m3u8") {
		return d.downloadM3U8(task)
	}
	// fallback 原本的下载器
	return d.downloadRegularFile(task)
}
