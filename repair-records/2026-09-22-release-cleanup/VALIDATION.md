# 验证记录

## 删除前核对

- 远程 Release：5 个。
- 有资产的 Release：`latest`（名称 v3.2.0）、`v3.2.1`、`v3.2.2`、`v3.2.4`。
- `v3.2.3`：0 个资产。
- 待删除资产：13 个，已逐一记录 asset ID 并下载到本地备份。
- 本地备份：`/Users/TT/Downloads/tal_downloader-release-assets-backup-20260922`。
- 备份校验：13 个文件的 SHA-256 已记录在 `BACKUP_MANIFEST.md`。

## 源码验证

```text
Ruby YAML 结构解析：PASS
git diff --check（工作流和记录文件）：PASS
ROLLBACK.sh shell 语法检查：PASS
source.patch 反向安全检查：PASS
source.patch SHA-256：32eca410d7fb2ef75784207d2e06a3d704c56ce18411a3b25f50a37af01b72a0
```

## 线上验证

删除动作完成后，应满足：

```text
所有 Release 的 assets 数量：0，已验证
Release 条目和标签：仍存在，已验证
远程剩余 asset API 对象：0
```

工作流合并后，应满足：

```text
PR / pull_request 运行：只上传 Actions artifact
release 或 workflow_dispatch 运行：允许上传 Release asset
```
