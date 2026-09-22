# macOS ARM 自动构建修复记录

- 日期：2026-09-22
- 基线：`2933c8ca0fbb82e3bbe92ff2ab96bd91afa3fed7`
- 工作分支：`codex/split-macos-arm-build`
- 范围：将 macOS ARM 构建从 Android 混合 job 中拆出，并让它自动上传 Release。

## 原因

原工作流把 macOS ARM、Android SDK 初始化和 Android 构建放在同一个 job。Android SDK 初始化失败时，macOS ARM 步骤会被全部跳过，导致 Release 没有 macOS ARM 资产。

## 行为变化

- 新增独立的 `Build_MacOS_ARM64` job，使用 `macos-15` GitHub-hosted arm64 runner。
- macOS ARM 使用标准 Fyne 打包路径，避免 `--release` 路径偶发卡住。
- Android 保留为独立 job，Android 失败不再阻断 macOS ARM、Windows 或 Linux 资产上传。
- macOS ARM 产物继续命名为 `tal_downloader_macos_arm64_<tag>.zip`，并自动上传到对应 Release。

## 修改文件

- `.github/workflows/build.yml`
- `.github/workflows/debug.yml`
- `repair-records/2026-09-22-macos-arm-build/*`

## 回滚

```sh
sh repair-records/2026-09-22-macos-arm-build/ROLLBACK.sh
```

脚本先执行反向补丁安全检查，只回滚本次工作流修改，并保留修复记录目录。
