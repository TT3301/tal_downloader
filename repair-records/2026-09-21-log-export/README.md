# 日志导出功能修改记录

- 日期：2026-09-21
- 基线：`a773e0777ed4a83344e440117a0cbbfc7c782d6c`
- 工作分支：`codex/add-diagnostic-logging`
- 范围：在课程页面增加“导出日志”按钮，并修正诊断日志导出一致性及脱敏覆盖。

## 行为变化

- “导出日志”位于“开始下载”左侧，点击后选择保存位置。
- 默认文件名为 `tal_downloader_diagnostics_日期-时间.jsonl`。
- 导出和日志写入共用互斥锁，避免导出文件以半条 JSON 事件结尾。
- 没有日志时显示明确提示，不生成空白日志。
- 增补非 JSON 文本，以及 `auth_key`、手机号、邮箱、账号等字段的脱敏。

## 修改文件

- `ui/course_select.go`
- `utils/diagnostic_log.go`
- `utils/diagnostic_log_test.go`

## 回滚

```sh
sh repair-records/2026-09-21-log-export/ROLLBACK.sh
```

脚本先执行反向补丁安全检查，只回滚本次源码变更，保留修改记录。
