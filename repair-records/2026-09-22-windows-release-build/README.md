# Windows Release 构建卡死修复记录

- 日期：2026-09-22
- 基线：`281df4ae2da6c394cfc66d0ad82e451c4d9e273c`
- 工作分支：`codex/fix-windows-release-build`
- 范围：修复正式 Windows 构建在 GitHub Runner 上卡在 Fyne `--release` 打包步骤的问题。

## 行为变化

- Windows Release 构建使用已验证能够完成的标准 Fyne 打包路径。
- 保留正式版本号、文件名和 Release 上传逻辑。
- 不改变应用代码和 Windows 产物的目标平台。

## 修改文件

- `.github/workflows/build.yml`

## 回滚

```sh
sh repair-records/2026-09-22-windows-release-build/ROLLBACK.sh
```

脚本先执行反向补丁安全检查，只回滚本次 Windows CI 修改，保留修改记录。
