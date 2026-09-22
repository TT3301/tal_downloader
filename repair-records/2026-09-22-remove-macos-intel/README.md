# 删除 macOS Intel 构建任务记录

- 日期：2026-09-22
- 基线：`99ff112e700c015f01a6cef4e035b5e8acfb20d1`
- 工作分支：`codex/remove-macos-intel`
- 范围：从正式 Release 工作流和 Debug 工作流删除 macOS Intel 构建任务。

## 行为变化

- `build.yml` 不再构建或上传 macOS x86_64 产物。
- `debug.yml` 不再运行 macOS Intel 调试构建。
- macOS ARM、Windows、Linux 和独立 Android job 保留不变。

## 修改文件

- `.github/workflows/build.yml`
- `.github/workflows/debug.yml`
- `repair-records/2026-09-22-remove-macos-intel/*`

## 回滚

```sh
sh repair-records/2026-09-22-remove-macos-intel/ROLLBACK.sh
```

脚本会先对两个工作流执行反向补丁安全检查，任一检查失败都不会修改文件。
