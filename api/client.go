package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsHenry35/tal_downloader/config"
	"github.com/itsHenry35/tal_downloader/utils"
)

type Client struct {
	httpClient *http.Client
	token      string
	userID     string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) SetAuth(token, userID string) {
	if token != "" {
		c.token = token
	}
	if userID != "" {
		c.userID = userID
	}
}

// returns token, userID
func (c *Client) GetAuth() (string, string) {
	return c.token, c.userID
}

func (c *Client) getDefaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent": config.UserAgent,
		"Referer":    "https://speiyou.cn/",
		"terminal":   config.Terminal,
		"version":    config.Version,
		"resVer":     config.ResVer,
	}
}

// nodh -> no default headers
func (c *Client) doRequest(method, urlStr string, body interface{}, headers map[string]string, nodh bool) (*http.Response, error) {
	var reqBody io.Reader

	if body != nil {
		switch v := body.(type) {
		case url.Values:
			reqBody = strings.NewReader(v.Encode())
		default:
			jsonData, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			reqBody = bytes.NewReader(jsonData)
		}
	}

	req, err := http.NewRequest(method, urlStr, reqBody)
	if err != nil {
		return nil, err
	}

	if !nodh {
		// Set default headers
		for k, v := range c.getDefaultHeaders() {
			req.Header.Set(k, v)
		}
	}

	// Set custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Add auth headers if available
	if c.token != "" {
		req.Header.Set("token", c.token)
	}
	if c.userID != "" {
		req.Header.Set("stuId", c.userID)
	}

	startedAt := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		utils.LogDiagnostic("http_request_error", map[string]interface{}{
			"method":      method,
			"url":         utils.SanitizeDiagnosticURL(urlStr),
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"error":       err.Error(),
		})
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.LogDiagnostic("http_response_read_error", map[string]interface{}{
			"method":      method,
			"url":         utils.SanitizeDiagnosticURL(urlStr),
			"status":      resp.StatusCode,
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"error":       err.Error(),
		})
		return nil, err
	}
	// Reset the response body so it can be read again later
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	utils.LogHTTPResponse(method, urlStr, resp.StatusCode, time.Since(startedAt), bodyBytes)

	return resp, nil
}

func requireHTTPSuccess(resp *http.Response) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var apiError struct {
		Message string `json:"message"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&apiError)
	message := strings.TrimSpace(apiError.Message)
	if message == "" {
		message = strings.TrimSpace(apiError.ErrMsg)
	}
	if message != "" {
		return fmt.Errorf("接口请求失败（HTTP %d）：%s", resp.StatusCode, message)
	}
	return fmt.Errorf("接口请求失败（HTTP %d）", resp.StatusCode)
}
