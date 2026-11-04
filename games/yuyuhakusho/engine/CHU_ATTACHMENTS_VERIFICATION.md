# 酎附件对象：实现前验证范围

目标为父人物 `+0x200` 的蓄力附件回调 `0x132de` 与 `+0x300` 的释放附件回调 `0x1361e`，恢复完整初始化、阶段分派、父对象关联、动画与清理。先保存原版完整输入／输出，再写测试并保留缺实现失败，最后编写独立 Go 规则。

可能失败的场景：把附件创建当作后续回调已完成；将父对象地址、平台席位、角色编号或目标位混用；忽略父动作／占用／生命／暂停条件；按错误阶段或回调内 JSR 的返回提前结束采样；丢弃 byte／word 写入后的旧字段与定点小数；对不匹配阶段自创回退；错误推进动画、位置或生命周期；遗漏形状、声音、调色板／属性和全局写入；原版对象删除被误作整块清零；把特殊动画数据误用为人物动画目录；只比较选取字段或重新编码的自洽性；观察器改变自然帧；每步回灌期望状态后声称连续生命周期；测试／缓存写到 worktree 外。

验证须比较完整附件、父人物、涉及的缓冲与计数，以及原版实际写入的全局状态。自然输入若有调整，先由未观察的固定原内核生成同输入基线；隔离调用明确声明私有输入。连续轨迹仅允许实际外部输入／父状态变化，不重新注入原版中间结果。未知子对象调度或呈现时序保持明确边界。

## 规则边界与已确认行为

独立接口为 `StepChuAttachment(state, parent, world, library)`，返回下一状态和 `Draw/ClearedBytes`。规则终点是进入 `0x1580` 精灵提交之前，或 `0x3556` 对象清理完成之后；不把 MD DMA／SAT 编码复制到规则层，也不把本阶段称为包含精灵提交的完整回调。原始指令见[附件核心](../../../docs/research/md-yuyuhakusho/static/chu-attachments-core.txt)，通用清理指令复用[投射物依赖](../../../docs/research/md-yuyuhakusho/static/chu-projectile-helpers.txt)。

蓄力附件使用独立 byte 倒计时。源 `+7c` 先按 signed word 右移 5 位：零值比较完整 word，之后的 1～7 只比较低 byte。每个只读序列有两条持续 2 步的记录和零时长终止符；终止时在同次调用内回到第一条。投影前复制父坐标，附件能量先加 16，再按 unsigned word 与父值比较，必要时重置帧计数。父角色 0／3 依父动画索引 4 保持，角色 9 依父姿态 `a6` 保持，其他编号没有该删除门槛。

释放附件每次调用都写生命 255，包括暂停／慢动作。非暂停时使用已恢复的通用动画时钟；完成值恰好为 1 或父姿态不为 `a7` 时清理。正常路径复制父坐标、提交动画／形状与声音，再投影。暂停和慢动作继续返回绘制请求，冻结动画、父状态删除判断及投影。

初始化后的图形集合为 `0xd8f1a`、图块基址为 `0x240000`。蓄力的动画集合字段实际保存 `0x13492..0x134bc` 八个倒计时序列标识；释放使用动画集合 `0xd92ca`、形状集合 `0xd9a3a`，姿态 26～32 各三帧，共 21 条帧记录。这些附件数据不属于已恢复的 11 个父人物目录；正式装载须另行合并。

通用清理在 size 为零时清 64 bytes，否则按 `size >> 4` 次写入 16-byte 块。已验证 size 0／16／31／64／128／256，包含部分清理后保留的后半对象字段。独立状态保留 256-byte 对象范围，拒绝超出该范围的清理；size 1～15 会令原版 DBF 计数回绕并写入大量别名内存，不作为正常附件规则覆盖。

## 已验证结果

- 自然规则阶段 177 次：蓄力 164、释放 13，174 次绘制请求、3 次删除。使用[原版特殊输入](testdata/chu-special-motion-inputs.json)，先由未观察的固定原内核生成同输入 3,600 帧基线；观察内核全部 RAM／视频一致。
- 原 CPU 隔离 10,877 次：蓄力 4,823、释放 6,054，包含 1,590 次删除。覆盖初始化与非零阶段、暂停／慢动作、父门槛、word 回绕／低 byte 别名、倒计时循环、通用动画完成、全部七种释放姿态和部分清理范围。
- 8 条单初态生命周期共 236 步，全部运行至删除。只更新声明的父坐标／能量／姿态及暂停／慢动作输入；不恢复子对象中间态，不额外推进外部时钟。4 条蓄力流共 164 步，4 条释放流共 72 步。
- 比较完整 256-byte 附件、256-byte 父人物、1,024-byte 形状区、四个计数、待处理声音与四槽队列；所有记录没有外部声音队列消费。隔离还检查完整 RAM 的范围外写入，均为零。
- 原生竞态和 Node/Wasm 的 3 项顶层测试通过。保留实现前实际缺少类型／函数的失败日志；测试副本仅优化了库不可变性检查的序列化范围与错误用例基底，覆盖和断言保持。

来源、夹具与红阶段／原生／Wasm 日志摘要见[验证记录](testdata/chu-attachments-validation.json)。无效类型、缺失库及不支持的清理范围均保持整体原子失败；两类只读动画库在调用后不变。

## 复跑

从 worktree 根目录采集原版规则：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_attachments.py \
  --out .local/md-yuyuhakusho/chu-attachments-replay
```

完整引擎就绪后运行 `go test -race -count=1 -run '^TestChuAttachments' ./games/yuyuhakusho/engine`。其他组件正在测试红阶段时，可从 `games/yuyuhakusho/engine` 使用以下同依赖范围独立复跑：

```sh
CHU_ATTACHMENT_ROOT=$(cd ../../.. && pwd)
mkdir -p "$CHU_ATTACHMENT_ROOT/.local/md-yuyuhakusho/tmp"
GOCACHE="$CHU_ATTACHMENT_ROOT/.local/go-cache" \
GOMODCACHE="$CHU_ATTACHMENT_ROOT/.local/go-mod" \
GOTMPDIR="$CHU_ATTACHMENT_ROOT/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json chu_attachments.go animation_records.go \
  animation_clock.go motion.go chu_attachments_test.go
GOOS=js GOARCH=wasm GOCACHE="$CHU_ATTACHMENT_ROOT/.local/go-cache" \
GOMODCACHE="$CHU_ATTACHMENT_ROOT/.local/go-mod" \
GOTMPDIR="$CHU_ATTACHMENT_ROOT/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  chu_attachments.go animation_records.go animation_clock.go motion.go chu_attachments_test.go
```

仍未覆盖世界级对象调度、精灵提交／资源装载、最终像素呈现或完整对局。父对象由本阶段的调用者提供，不能把声明的独立生命周期视为完整人物／世界时间线。
