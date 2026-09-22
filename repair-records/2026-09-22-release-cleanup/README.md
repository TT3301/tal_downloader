# Release 资产清理与 PR 上传隔离记录

- 日期：2026-09-22
- 基线：`3b93963bf40171cc7878a3e2d0196fa4352f2916`
- 工作分支：`codex/no-pr-release-upload`
- 目标仓库：`TT3301/tal_downloader`

## 处理范围

- 删除远程 Release `v3.2.0`、`v3.2.1`、`v3.2.2`、`v3.2.4` 下的全部 13 个资产。
- `v3.2.3` 本来没有资产，因此没有删除动作。
- 保留 Release 条目、标签和说明，不删除 Release 本身。
- 资产删除前已备份到：
  `/Users/TT/Downloads/tal_downloader-release-assets-backup-20260922`

注意：GitHub API 中历史 `v3.2.0` Release 的标签名实际为 `latest`，回滚脚本按真实标签 `latest` 恢复。

## 工作流修改

`.github/workflows/build.yml` 的 5 个 Release 上传步骤现在都要求：

```yaml
if: github.event_name == 'release' || github.event_name == 'workflow_dispatch'
```

因此后续 PR 的 `Debug` 工作流只会上传 Actions artifact，不会上传或覆盖 GitHub Release 资产。正式 Release 事件和明确的手动 Build 仍可上传资产。

## 回滚

代码和工作流回滚：

```sh
sh repair-records/2026-09-22-release-cleanup/ROLLBACK.sh
```

脚本会先反向应用源码补丁，再从本地备份恢复 13 个远程 Release 资产；需要本机已登录 `gh`，且备份目录仍存在。
