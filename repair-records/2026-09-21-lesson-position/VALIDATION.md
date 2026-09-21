# 验证记录

## 已执行验证

```text
gofmt：PASS
go test ./...：PASS
go test -race ./...：PASS
go vet ./...：PASS
git diff --check：PASS
调班课节顺序回归测试：PASS（当前班第4至5讲，历史班第3、2、1讲，最终排序为第1至5讲）
服务端 pos 覆盖 ListIndex 测试：PASS
source.patch 反向应用安全检查：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；五个源码文件恢复为基线状态，记录目录保留
```

macOS 链接器输出了已有的 `ignoring duplicate libraries: '-lobjc'` 警告，不影响测试通过。

## 补丁校验

```text
source.patch SHA-256：77e3ee5f84c8af32824311e92682d8adc1ec4ea61f3eb4aa2e048d591c4a1f96
```

## 验证边界

- 本地测试使用模拟接口，不访问用户账号。
- Windows 原生界面和实际下载由 GitHub Actions 构建产物进行人工验证。
