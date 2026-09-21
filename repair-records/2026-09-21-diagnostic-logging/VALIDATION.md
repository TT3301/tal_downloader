# 验证记录

## 已执行验证

```text
Go：go1.27.1 darwin/arm64
git diff --check：PASS
go test ./...：PASS
go test -race ./...：PASS
go vet ./...：PASS
日志 URL 脱敏测试：PASS
日志 JSON 响应脱敏测试：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；源码恢复为基线，新增日志文件和测试文件被移除，记录目录保留
```

`source.patch` 属于可审阅的补丁文本，源码的 `git diff --check` 不包含该生成文件。

## 补丁校验

```text
source.patch SHA-256:
50b60627e8df8ac88cac0f0c154231bdfcb8ab3f1c909e21692b64ec810d857d
```

## 验证重点

- 课程列表响应保留 `stdCourseId`、课程名、科目、课节数和来源信息，便于核对服务端数据与界面去重结果。
- `stuId`、token、密码、Cookie、授权字段和签名媒体 URL 参数不会写入日志。
- HTTP 状态错误、网络请求错误、响应读取错误和课程 JSON 解析错误都有对应日志事件。
- 日志写入使用追加模式和互斥锁，多个 API 请求并发时不会交错写入同一行。

## 验证边界

- 本地测试未访问用户账号或线上接口。
- Windows GUI 的最终日志路径和实际响应内容需要由新版客户端运行后确认。
- 本次不修改课程去重规则和下载逻辑。
