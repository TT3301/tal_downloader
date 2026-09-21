# 客户端归属信息修复记录

- 日期：2026-09-21
- 基线：用户仓库 `main` / `dc75f9ae14a8d5dd958d611b1bd25aa3d85c7562`
- 工作分支：`codex/update-owner-metadata`
- 发布目标：`v3.2.2`
- 范围：修正客户端“关于”窗口的作者、GitHub 和反馈问题链接；不改变下载逻辑。

## 修改内容

`constants/constants.go` 中的归属信息由上游作者和仓库改为用户仓库：

- 作者：`TT3301`
- GitHub：`https://github.com/TT3301/tal_downloader`
- 反馈问题：`https://github.com/TT3301/tal_downloader/issues`

GitHub Actions 构建时仍会按提交自动写入版本号和编译时间；本次修复不会把版本号硬编码到源码中。

## 交付物

- `source.patch`：可审阅、转移或反向应用的源码补丁。
- `VALIDATION.md`：本地验证和回滚验证记录。
- `ROLLBACK.sh`：带安全检查的可运行回滚脚本。

## 回滚

在未产生额外源码冲突时运行：

```sh
sh repair-records/2026-09-21-owner-metadata/ROLLBACK.sh
```

脚本只反向应用本次源码补丁，保留本修复记录目录。
