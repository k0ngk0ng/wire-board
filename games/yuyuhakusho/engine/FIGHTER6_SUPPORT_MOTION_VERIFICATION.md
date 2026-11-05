# 阵支持动作：实现前边界

本阶段恢复来源 6 的支持槽 `0E/0F/11/13/37`，止于共享尾段前 `15AFC`，并提供 MAIN `38` 使用的 `18682` 构造。主体动作 `33/34/35/36/38` 与统一入口由另一模块维护。接口为 `StepFighter6SupportMotion(Fighter6MotionState, JumpParameters)`；状态统一使用 `ChuSpecialMotionState` 加 `StunTicks uint16`，对应已有 canonical `WorldObject.StunTicks(+84)`。

`18682` 使用 Body 已有 `Auxiliary7E/80`，不新增字段：设置 action `0C`、pose `2B`、EnabledStages `0`，把 `Auxiliary80` 的 word 复制到 `Auxiliary7E`，再执行原动作重置。共享 helper 命名 `enterFighter6Exit(body *FighterBodyState)`，只由本支持模块定义，MAIN 直接调用。它不把人物降格为普通空中状态，不修改 StunTicks、重力、速度或 ExtraJump。

生产前必须先保存失败场景与原 CPU 字节：

- 来源 6 的资源／参数／只读帧表从真实初始化取得，不能仅改其他角色 tag；自然观察匹配同输入唯一未观察原 core 基线，恢复包括 `refresh_cycles`。
- `0E` 复用已恢复 F0 相同入口；完成时区分地面高 word 相等和空中，非完成只在地面／ticks0分配单次尘土；位移使用 Facing bit0。
- `0F` 复用 Chu 同入口；移动后，FrameIndex 非零且 ticks0 才对完整横速算术减半。不能合并成 `0E` 或在位移前减半。
- `11` 复用 F10 八项退避表：0／6／7 不动，1..5 退避，完成时按地面／空中恢复；非法实际表项原子拒绝，不称原版 no-op。
- `13` 复用 F3 同入口：FrameIndex0 不移动；ticks0先尘土后位移，FrameIndex≥2再减半；完成按 Flags.bit2恢复站立／蹲下，保留有符号 long 回绕。
- `37` 只消费 Completed 恰为1，随后无条件执行 `1860C`：清 ExtraJump／VX／VY、恢复参数重力并进入 `02/07`、mask14，即使 Y 高 word 与 Ground 相等。其他非零完成 byte 保持 no-op，不套用有地面判断的恢复。
- `18682` 的 Aux80→Aux7E 逐 word复制，极值／负值和原 StunTicks 独立验证。ResetActionState 的 FrameIndex／Duration／Ticks／Completed、Aux64／ActionFlags／Aux77／Team/BaseTeam 顺序按原保留，其他字段不动。
- 控制器扩展 byte、能量小数、Power、AlternateVelocity、父三子、FX/extras、shape缓冲、声音与RNG完整比较；未读取的游标／字段不制造额外错误。
- 缺库／非法池游标或帧表的晚期错误必须原子返回，不泄露先前写入；单侧分配失败／满池是正常结果，游标与旧槽字段保留。
- 每条连续流只初始化一次，原动画时钟和各自当前 pose／index 的只读时序刷新推进，不回灌中间人物、FX或RNG；私有调用另核对全RAM及返回SP，自然IRQ／队列消费者单独留证。

先原 CPU 自然／边界／真实时钟流，再写支持函数与 helper 的缺实现测试，真实红灯之后才加入生产代码。复用同一入口仍需新的来源 6 上下文，不能把旧角色验收直接改名。静态资料使用[直接 Ghidra 有限清单](../../../docs/research/md-yuyuhakusho/static/fighter6-support.txt)，REA不支持raw68000，不称为REA反编译。本阶段只新增支持模块与研究证据，不改旧Go、bundle、PROGRESS、其他报告或服务资产。

## 普通与私有材料

真实普通来源 6 的 3,602 个视频步被动观察逐步匹配共享原内核参考。实际自然返回 34 次：共享 `11` 退避 33 次，`18682` 完整构造一次；其他支持槽没有在该输入中自然观察，不把私有分支写成自然覆盖。

| 原 CPU 范围 | 数量 |
| --- | ---: |
| `0E`／`0F` 的完整位移／完成边界 | 各 330 次 |
| `11` 的八项表及完成边界 | 192 次 |
| `13` 的门槛／符号／顺序／完成边界 | 1,806 次 |
| `37` 的严格完成与无条件空中恢复 | 72 次 |
| `18682` 独立完整 RTS | 270 次 |
| 单初态真实时钟流 | 20 条、2,574 个阶段 |

边界合计 3,000 次。连续流包含原时钟 858 次、支持阶段 858 次、只读时序刷新 858 次；后者只根据双方各自当前 pose／index 读取同一记录并写 Duration／Flags，不计为 CPU 指令执行。因此实际 CPU 调用为 1,716 次，未回灌期望位置、时钟、FX 或 RNG。各流最终恢复站立或原无条件空中状态，不以它们代替完整人物／世界更新。

私有初态来自真实来源 6 的第 2,400 视频步 checkpoint，资源／参数指针保留其初始化来源；每次恢复显式包含 `refresh_cycles`。比较完整父／三子 1,024 bytes、FX 池 2,048 bytes／extras、shape 缓冲 1,024 bytes及游标、声音与RNG。私有调用另查完整64KiB RAM与返回SP；独立构造按真实RTS返回SP+4，动作阶段保持SP。普通、边界及连续材料均无独立队列消费者、RNG调用、IRQ或未解释范围外改写。

支持函数与 helper 的[真实红灯](testdata/fighter6-support-motion-red.jsonl)明确报告本阶段缺少 `StepFighter6SupportMotion`／`enterFighter6Exit`；[先行记录](testdata/fighter6-support-motion-test-first.json)在生产文件不存在时绑定测试与CPU材料。三项普通／私有／clock／原子检查首次正常 package native `-race` 10.957秒、Node/Wasm 2.643秒通过，无规则修正。特殊自然输入就绪后，使用同一先行自然断言独立追加，不改原命令程序或MetaInput制造覆盖。

## 特殊自然与最终结果

[合法真人特殊输入](testdata/fighter6-special-inputs.json)的3,602视频步观察逐步匹配[唯一原core基线](testdata/fighter6-attacks-special-reference-manifest.json)。获得115次完整返回：`13`39次、`37`75次、`18682`一次。该构造在第2,848步返回`19D90`，属于共用基础退出；不把它称为MAIN38的退出自然覆盖，后者由主任务延长世界流程另验。

第1,850步的`13`回调跨两个RAM IRQ，均进入`FFFFF000`，分别在`FFF09A`／`FFF0A8`返回。IRQ前后RAM分别有114／1个byte变化，按时序串联后解释了51个规则输出区外的最终byte；[特殊自然夹具](testdata/fighter6-support-motion-special-oracle.json.gz)保留完整原差异。测试逐项核对写入链，未解释变化为0，没有对规则字节清零或回灌期望值。独立队列消费者及RNG调用均为0。

最终四项正常package测试全部通过：native `-race`10.990秒，Node/Wasm2.682秒，完整engine `go vet`通过。支持生产文件自首次实现未改，状态和helper接口冻结；不需要把不可达的人类命令伪造成其他MAIN槽的自然覆盖。当前源码、原CPU来源、两条输入／参考、日志和清理SHA绑定见[最终验证报告](testdata/fighter6-support-motion-validation.json)。

## 复跑

从worktree根目录执行，复用已保存原参考，不重复控制组：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_support_motion.py \
  --out .local/md-yuyuhakusho/fighter6-support-motion-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_support_motion.py \
  --out .local/md-yuyuhakusho/fighter6-support-motion-special-replay --natural-only \
  --prefix fighter6-support-motion-special \
  --inputs games/yuyuhakusho/engine/testdata/fighter6-special-inputs.json \
  --reference games/yuyuhakusho/engine/testdata/fighter6-attacks-special-reference-frames.json \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter6-attacks-special-reference-manifest.json

mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestFighter6Support'

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine -run '^TestFighter6Support'
```

本阶段只交付所列支持动作／退出构造闭合边界，未修改MAIN实现、旧Go、bundle、PROGRESS、其他报告或服务资产，没有提交／发布／上传。完整人物／世界、恢复和平台E2E继续由主任务组合验收。
