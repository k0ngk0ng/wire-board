# 幻海两种子对象：实现前验证范围

本阶段只恢复来源 0 的散射弹 `12C96` 和释放光效 `134C2`，分别停在原 `1580` 绘制请求或完成 `3556` 清除之后。不调度父体、其他对象或碰撞消费者，不包含硬件精灵提交。原始静态资料来自[直接 Ghidra 的 68000 指令](../../../docs/research/md-yuyuhakusho/static/fighter0-projectile.txt)；REA 不支持本 raw 68000 输入，不称为 REA 反编译。自然执行和静态指令的覆盖分别报告。

在实现代码之前，先为以下失败场景保留原 CPU 输出与测试失败：

- 两种初始化都保留未写字段；散射弹不能额外清 FrameIndex／Flags／Completed，释放光效不能额外清 Duration／Completed。
- 初始化先于暂停。暂停／慢动作只请求绘制，不清震动、不推进计时、不移动、不投影。
- 散射弹先计时，再在 Completed 恰等于 1 时删除；其他非零 byte 不能误作完成。完成删除先于运动和冲击 FX。
- 位置按 signed 16.16 long 回绕；落地使用纵速 long 符号与坐标高 word；左右边界含 word 加减回绕和严格比较。
- 落地或接触 bit3／bit4 使用 `1FDBE` 冲击分配后删除；边界／动画完成只删除。FX 空池／单槽／满池、游标推进、word 坐标写入保留旧小数均须核对。
- 散射弹使用完整动画、四类 shapes、声音队列及投影顺序，不能从 expected 写回数据或在单初态连续流中重置中间帧。
- 释放光效按 Power 的 signed word 右移选择八组序列；第 0 项比较整个 word，1..7 比较低 byte，其他值保留旧 identity。
- 释放光效的两字节帧是递减倒计时；终止项写 GraphicParameter／Ticks 后立即删除，不能循环或跟随父体，也不提交 shapes／声音。
- 原清除按 Size 的 16-byte 前缀向下取整，Size=0 使用 64；部分清除后必须保留前缀外字段。
- 缺少动画／序列、序列终止非法、非法游标／清除范围以及晚期提交失败必须原子返回，不泄露初始化、运动、FX 或缓冲写入。
- 每次原 CPU 调用保留子对象、父体、完整 FX 池、四组缓冲、RNG 与声音的原始字节，并核对全 RAM 改写范围。自然采集必须与无钩子原内核逐视频步 RAM／画面一致；独立采样输入与自然行为不可混报。

测试只在保存原 CPU 夹具、编写断言并记录缺实现失败之后加入规则。后续完整世界、父子同时运行、新建对局、平台输入和呈现另行验收。

## 实际证据与规则边界

原菜单与原按钮序列从冷启动进入幻海对局。旧 `source0-next-inputs.json` 的前 3,602 视频步已经完成双内核比对，但未触发这两种新子对象，因此没有把该流列为新子对象自然覆盖。专属[真实输入序列](testdata/fighter0-projectile-inputs.json)保留前 1,800 步的菜单输入，随后通过「下、松开、下、B」触发散射，通过「下、右下、右、B」蓄力并松开，获得两种真实自然回调。没有为自然取证改写 RAM 或强行写入动作状态。

专属流使用固定无钩子原内核和被动钩子内核分别运行 3,602 个视频步，逐步核对完整 RAM／画面摘要。第 3,600 步之后只排空挂起回调，不接受新调用。[规范参考帧](testdata/fighter0-projectile-reference-frames.json.gz)解压后有 3,602 条 JSON 记录，配套[来源绑定](testdata/fighter0-projectile-reference-manifest.json)可供父动作和完整人物组合复用。

| 范围 | 散射弹 `12C96` | 释放光效 `134C2` | 合计 |
| --- | --- | --- | --- |
| 自然完整回调 | 34 | 13 | 47 次，含 4 次删除 |
| 原 CPU 隔离边界 | 2,664 | 3,173 | 5,837 次，含 966 次删除 |
| 单初态生命周期 | 24 条／192 步 | 20 条／280 步 | 44 条／472 步，全部到原清除出口 |

每个完整返回核对 256-byte 子对象、父对象、2,048-byte FX 池、1,024-byte shape 缓冲、四个缓冲游标、FX 游标、RNG、待提交声音和四槽队列。全部自然／隔离／连续调用均无独立声音消费者事件或未解释 RAM 改写；私有调用的全 RAM 比较仅允许已声明输出区与本次私有栈。中间生命周期只施加明确声明的接触、暂停和慢动作输入，不回灌原中间状态、帧记录、色盘或 RNG。

散射弹保留 `Completed == 1` 的严格判断：动画完成先清除，落地／接触先分配固定 `18` 冲击 FX 再清除，越界直接清除。释放光效只选择短序列、递减倒计时并投影，不追踪父体、不改生命、不提交 shape 或声音。两者复用 `ChuProjectileState` 的 canonical 字段所有者及已验证前缀清除；没有重复存储或运行原代码地址。

25 条只读动画记录来自 `D92CA/D9A3A` 的姿态 `0..8、33..35`，其中 `0..3、8` 的 11 条是本阶段补充。所有记录分别用左右朝向交给原 CPU 的完整散射回调验证，共 50 次，已包含在隔离调用总数中，不能再次相加。释放短序列为 `135EE+6*n` 的八组，每组三个两字节帧；终止项立即删除。纯[语义数据](../data/fighter0-projectile-rules.json)保留 25 条动画和八组序列，不包含原 CPU 用例、RAM 或指令字节。[导出报告](../data/fighter0-projectile-rules-manifest.json)绑定其证据。共享 `1313E` 在来源 0 父体上下文中的完整运行仍由世界组合验收证明，此处不冒充该组合验收。

## 测试与复跑

[缺实现失败](testdata/fighter0-projectile-red.jsonl)明确报告本阶段自己的 `Fighter0ProjectileLibrary/State` 等符号缺失；[测试先行记录](testdata/fighter0-projectile-test-first.json)在实现文件不存在时绑定了原 CPU 材料和测试。当前三个顶层测试分别核对完整返回、单初态连续删除及原子失败；native 使用 `-race`，Wasm 使用相同原始字节与断言。精确来源与日志摘要见[验证报告](testdata/fighter0-projectile-validation.json)。这些是封闭规则回调验收，不是浏览器或平台房间 E2E。

从本 worktree 根目录执行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter0_projectiles.py \
  --out .local/md-yuyuhakusho/fighter0-projectile-natural
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/export_fighter0_projectile_data.py \
  --from .local/md-yuyuhakusho/fighter0-projectile-natural

mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestFighter0Projectile'

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine -run '^TestFighter0Projectile'
```

ROM、模拟器与私有研究状态仍只用于开发对照；本阶段没有改父动作、世界调度、既有语义包或平台运行时，没有发布、上传或提交。非法清除长度／表项只在独立 API 边界原子拒绝，不执行原 CPU 对损坏地址的跨槽改写。
