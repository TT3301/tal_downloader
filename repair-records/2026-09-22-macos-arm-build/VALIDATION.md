# 验证记录

## 已执行验证

```text
Ruby YAML 结构解析：PASS
git diff --check：PASS
go test ./...：PASS
source.patch 反向应用安全检查：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；build.yml 恢复为基线状态，记录目录保留
actionlint：未执行（本机未安装）
```

## 目标平台验证边界

- macOS ARM 构建需要在 GitHub Actions 的 `macos-15` arm64 runner 上验证。
- Android 构建已与 macOS ARM 解耦；本次不改变 Android 产物命名。
- 合并后应手动运行 Build workflow，输入已有 Release 标签（例如 v3.2.4），确认 macOS ARM ZIP 出现在 Release 资产中。

## 补丁校验

```text
source.patch SHA-256：46fc2eabf0bf6ac4fae991cb8632697305e313dbcc82fe028f1f3259286cf48b
```
