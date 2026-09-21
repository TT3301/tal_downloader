# 手动发布 Release 工作流修改记录

- 日期：2026-09-21
- 基线：`9226b8349ee2495ed1151d45c498226bf2cded14`
- 工作分支：`codex/add-diagnostic-logging`
- 范围：增加 GitHub Actions 手动发布入口，不修改现有客户端源码与构建脚本。

## 使用方式

1. 将本分支 PR 合并到 `main`。
2. 打开 Actions，选择 **Publish Release**。
3. 点击 **Run workflow**，输入新版本标签，例如 `v3.2.3`。
4. 工作流从 `main` 创建 GitHub Release；现有 **Build** 工作流随后自动构建并上传 Windows 客户端。

工作流拒绝格式不正确或已存在的版本标签，并可选择标记为预发布版本。

## 修改文件

- `.github/workflows/manual-release.yml`
- `.github/workflows/build.yml`（显式授予上传 Release 附件所需的 `contents: write` 权限）

## 回滚

```sh
sh repair-records/2026-09-21-manual-release/ROLLBACK.sh
```

脚本先进行反向补丁安全检查，只回滚本次工作流变更，保留修改记录。
