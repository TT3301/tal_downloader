# 验证记录

```text
Ruby YAML 解析 manual-release.yml：PASS
Ruby YAML 解析 build.yml：PASS
actionlint v1.7.7 manual-release.yml：PASS
actionlint v1.7.7 build.yml：PASS
源码与记录文件 git diff --check：PASS
source.patch 反向应用安全检查：PASS
临时 worktree 执行 ROLLBACK.sh：PASS；两个工作流文件恢复为父提交状态，记录目录保留
```

## 补丁校验

```text
source.patch SHA-256:
1b354a39df55097937723f81ed058d9a8affdd829f841225707dc4ca1bdfa8b3
```

## 行为检查

- 发布源固定为默认分支 `main`，不会把未合并的 PR 分支直接发布。
- 版本标签必须以 `v` 开头并包含三段数字版本号。
- 本地或远端已存在同名标签时停止，不覆盖旧 Release。
- 创建 Release 使用仓库内置 `GITHUB_TOKEN`，不需要新增个人访问令牌。
- Release 发布事件继续触发现有 `Build` 工作流；该工作流现在显式具备上传附件所需的写权限。

## 验证边界

- 不会在验证过程中实际创建标签或 GitHub Release。
- 工作流合并到 `main` 后才会出现在默认分支的 Actions 手动运行列表中。
- Release 创建后的多平台编译继续由现有 `.github/workflows/build.yml` 负责。
