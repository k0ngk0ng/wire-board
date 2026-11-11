# 旧控制器配置兼容

赛车动作从 11 项增至 18 项。旧版 `readControllerProfile` 直接按当前动作数校验存储配置，导致保存过旧键位的用户读取单游戏或全部控制器设置时返回 HTTP 500。

本轮先用真实 HTTP 保存旧模型配置，再使用同一 SQLite 数据库重启新模型，分别重现通用 2→5 和赛车 11→18 动作的失败，随后实现读取时补齐。GET 只补响应，不写回数据库；用户保存时仍使用原修订号并通过完整校验。

保留已有主键、替代键、显式空绑定、设备选择、死区、连发速度及修订号。新增默认键遇到已有绑定时留空。已有设备只对 `standard:` 尝试填入未占用的默认物理输入；非标准设备不猜按钮号。完整动作配置中的稀疏实体绑定保持原样；未知动作、未知 JSON 字段、无效或重复绑定及坏格式仍拒绝。

`controller_evolution_e2e_test.go` 验证 GET 前后 SQLite BLOB 和 revision 不变，正常保存 revision 增一，旧 revision 返回 409。全部共享控制器 HTTP 回归通过，耗时 2.486 秒：

```sh
. scripts/env.sh
go test ./internal/server -run 'Test(Controller|PhysicalController)' -count=1 -timeout=5m -v
```

本地红／绿日志及机器摘要位于 `.local/racing-nfs-backend/controller-evolution-before.log`、`controller-regression.log`、`controller-evolution-summary.json`。修复仅涉及共享控制器读取及 E2E，不改变已通过浏览器验收的赛车前端、规则、资源或构建。
