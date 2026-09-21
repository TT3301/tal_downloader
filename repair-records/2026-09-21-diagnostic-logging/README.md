# 诊断日志功能修复记录

- 日期：2026-09-21
- 基线：用户仓库 `main` / `5e32ad5ef14ef8490f5c9f3b7061834139573f2d`
- 工作分支：`codex/add-diagnostic-logging`
- 范围：增加本地诊断日志，记录接口响应、课程原始数据摘要、去重结果和错误；不改变接口请求参数或下载逻辑。

## 日志内容

- `app_start`：版本和日志路径。
- `http_response`：请求方法、脱敏 URL、状态码、耗时、响应大小、响应哈希和脱敏响应体。
- `http_request_error` / `http_response_read_error`：网络请求或读取响应失败。
- `course_list_page`：每页课程数量及服务端返回的课程字段。
- `course_list_merged`：去重前数量、界面显示数量、主课程及历史来源。
- `course_list_error`：课程接口请求、HTTP 状态或 JSON 解析错误。

日志文件为 JSON Lines，保存在 Fyne 应用数据目录的 `diagnostics.jsonl`。程序不会记录请求头、密码、token、Cookie、授权信息或签名 URL 参数；日志功能失败也不会影响下载。

## 修改文件

- `utils/diagnostic_log.go`
- `utils/diagnostic_log_test.go`
- `api/client.go`
- `api/course.go`
- `main.go`

## 回滚

```sh
sh repair-records/2026-09-21-diagnostic-logging/ROLLBACK.sh
```

脚本带反向应用安全检查，只回滚本次源码文件，保留本记录目录。
