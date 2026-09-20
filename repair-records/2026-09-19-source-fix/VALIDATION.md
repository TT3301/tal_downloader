# 验证记录

## 环境

- macOS arm64
- 临时工具链：Go 1.20.14 darwin/arm64
- 官方压缩包 SHA-256：`6da3f76164b215053daf730a9b8f1d673dbbaa4c61031374a6744b75cb728641`
- 未执行 `go build`、`fyne package` 或任何客户端打包命令

## 已执行验证

```text
git diff --check
结果：PASS

go test ./...
结果：PASS（包含课程转班分组与历史来源回退新增测试）

go test -race ./...
结果：PASS

go vet ./...
结果：PASS

git apply --reverse --check repair-records/2026-09-19-source-fix/source.patch
结果：PASS

在临时克隆中先应用 source.patch，再执行 ROLLBACK.sh
结果：PASS，回滚后无任何 tracked file 差异
```

当前 `source.patch` SHA-256：

```text
faa127f51497b5857613d8bf92c9d5b80d770aa58c8b850332a6d14f728b23c2
```

测试覆盖：

- 调班产生的同名、同科目、同课节数课程记录合并，并保留不同课程/导师来源。
- 合并课程只按 task 三元组、`liveId` 或双方唯一标题建立历史来源回退，不再把不同课节按数组位置合并。
- 两个来源相同位置但标题与标识不同的课节保持独立，防止静默下载错片。
- 课节分页去重、稳定顺序和服务端课节标题解析。
- 延伸课程使用解析后的 `realRecordId` 请求录像资源。
- 录像 definitions 在 100 次选择中保持确定结果。
- Range 正常、服务器忽略 Range、错误 Range 三种下载路径。
- 不完整分段文件自动删除。
- HLS 主列表最高带宽选择、相对 URL 和查询参数解析。
- HLS 任一分片连续失败三次时，任务失败且不留下伪完成文件。
- 下载任务队列只消费一次。
- 失败任务可由“继续”重新启动，完成中的任务不能被重复启动。

## 验证边界

- 所有网络测试均使用本地 `httptest` 模拟服务，没有访问用户账号或 TAL 线上接口。
- 未编译或运行 GUI 客户端。
- 已通过 Windows v3.2.0 运行态只读内存检查确认重复记录结构；未输出或保存账号 ID、token、Cookie、密码。
- TAL 网络诊断包已确认视频 CDN 域名，但 HTTPS ETL 不包含明文 API 响应；线上新增字段、鉴权头和服务端版本要求仍未端到端实测。
- 加密 HLS 目前会明确报错并拒绝生成损坏文件，尚未实现密钥解密；是否需要支持要由真实 TAL 抓包确认。
