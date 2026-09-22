# Android SDK JDK 版本修复记录

- 日期：2026-09-22
- 基线：`e34152afa3a76ca3d116b766f3eb75576f4b40f2`
- 工作分支：`codex/fix-android-sdk-jdk`
- 范围：修复 GitHub Actions Android SDK 初始化因默认 Java 11 导致的失败。

## 原因

最近一次 Android job 的日志显示，`android-actions/setup-android@v3` 调用 `sdkmanager` 时报告：当前命令行工具要求 JDK 17 或更高版本，但 Runner 检测到 Java 11.0.32。

## 修改

在正式 Build 和 Debug 工作流的 Android job 中，在 Android SDK 初始化前增加：

```yaml
- uses: actions/setup-java@v6
  with:
    distribution: temurin
    java-version: '17'
```

`setup-java` 会设置 `JAVA_HOME` 和 `PATH`，供后续 `sdkmanager` 使用。

## 修改文件

- `.github/workflows/build.yml`
- `.github/workflows/debug.yml`
- `repair-records/2026-09-22-android-sdk-jdk17/*`

## 回滚

```sh
sh repair-records/2026-09-22-android-sdk-jdk17/ROLLBACK.sh
```
