# 验证记录

## 已执行验证

```text
Go：go1.27.1 darwin/arm64
gofmt：PASS
go test ./...：PASS
go test -race ./...：PASS
go vet ./...：PASS
源码与记录文件 git diff --check：PASS
导出内容一致性测试：PASS
非 JSON 文本敏感值脱敏测试：PASS
JSON 扩展敏感字段脱敏测试：PASS
source.patch 反向应用安全检查：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；三个源码文件恢复为父提交状态，记录目录保留
```

macOS 链接器输出了已有的 `ignoring duplicate libraries: '-lobjc'` 警告，不影响测试通过。

## 补丁校验

```text
source.patch SHA-256:
e7d1ec698e8d8d0c3cb4e07e20f27958e1b5f382dc3c84cf3852ee12059f9878
```

## 验证边界

- 本地测试不访问用户账号或线上课程接口。
- Windows 原生保存对话框与按钮最终布局由 GitHub Actions 构建产物进行人工界面验证。
- 本次不改变课程去重和下载映射逻辑。
