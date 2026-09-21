# 验证记录

## 修改件

- `constants/constants.go`
- `repair-records/2026-09-21-owner-metadata/README.md`
- `repair-records/2026-09-21-owner-metadata/VALIDATION.md`
- `repair-records/2026-09-21-owner-metadata/ROLLBACK.sh`
- `repair-records/2026-09-21-owner-metadata/source.patch`

## 已执行验证

```text
Go：go1.27.1 darwin/arm64
git diff --check：PASS
go test ./...：PASS
go test -race ./...：PASS
go vet ./...：PASS
git apply --reverse --check repair-records/2026-09-21-owner-metadata/source.patch：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；仅 `constants/constants.go` 恢复为基线内容，记录文件保持不变
```

## 预期结果

- 客户端“关于”窗口作者显示 `TT3301`。
- “GitHub”按钮打开 `https://github.com/TT3301/tal_downloader`。
- “反馈问题”按钮打开 `https://github.com/TT3301/tal_downloader/issues`。
- 下载逻辑和网络请求逻辑无变化。

## 验证边界

- 本地测试验证源码构建和常量变更；未在本机运行 Windows GUI。
- Windows EXE 将由 GitHub Actions 在合并到 `main` 后构建，再作为 `v3.2.2` Release 附件发布。

## 补丁校验

```text
source.patch SHA-256:
9da5a7aff4392b4c9cdb39ec57eaae9b35517654391b1c96a1777e34e2b8bf48
```
