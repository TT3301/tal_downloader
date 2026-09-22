# Release 构建手动触发修复记录

- 日期：2026-09-22
- 基线：`f6492e854d1d47aba1f877cc9401370c0700e1e7`
- 工作分支：`codex/manual-release-build-dispatch`
- 范围：允许正式 Build 工作流手动接收 Release 标签，修复 `GITHUB_TOKEN` 创建 Release 后不会自动触发 `release` 事件工作流的问题。

## 行为变化

- 保留 `release.published` 自动构建入口。
- 增加 `workflow_dispatch` 入口和必填 `version` 参数。
- 手动构建时使用输入的 Release 标签；Release 事件时继续使用 `github.event.release.tag_name`。
- Linux、Windows、macOS 和 Android 的构建文件名、版本信息及 Release 上传目标统一使用同一个标签。

## 修改文件

- `.github/workflows/build.yml`

## 回滚

```sh
sh repair-records/2026-09-22-build-dispatch/ROLLBACK.sh
```

脚本先执行反向补丁安全检查，只回滚本次 CI 修改，保留修改记录。
