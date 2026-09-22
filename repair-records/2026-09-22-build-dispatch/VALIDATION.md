# 验证记录

## 已执行验证

```text
YAML 结构解析：PASS（Ruby Psych）
actionlint：未安装，未执行
git diff --check：PASS
workflow_dispatch 标签表达式覆盖：PASS（34处使用 release tag 或手动输入回退）
source.patch 反向应用安全检查：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；build.yml 恢复为基线状态，记录目录保留
```

## 补丁校验

```text
source.patch SHA-256：f3721ebb7a310d252eaeda21d362d6a6a300c9c3c3b8d8b62713573b20aefb96
```

## 验证边界

- 本次修改只影响 GitHub Actions 触发入口和 Release 标签解析。
- 实际 Windows 构建将在合并后以 `version=v3.2.4` 手动触发并核对附件。
