# 来源 7「树」MAIN 验证

`StepFighter7Motion` 已恢复 `39..3d` 及 `0f/11/13/14/45` 共享动作。冻结的原版 CPU 对照、单初态时钟轨迹、失败原子性测试均通过正式包的原生竞态检查和 Node-WebAssembly 检查。范围止于共享动作尾入口 `15afc`，不据此宣称完整角色或世界验收完成。

## 原版证据

本轮复用主任务唯一的合法来源 7 菜单输入和 3,602 帧无钩子控制。观察核心逐帧完整 RAM／画面与控制一致，`2400` 帧的实际 `8400` 父体来源为 7。动画集为 `850330`，形状集为 `853658`，运动参数源为 `75248`；未将其他来源父体重新标记为来源 7。

| 动作 | 入口 | 普通自然调用 | dirty 边界调用 | 单初态轨迹中的 MAIN 调用 |
| --- | --- | ---: | ---: | ---: |
| `0f` | `1ca9c` | 0 | 294 | 42 |
| `11` | `1cb08` | 91 | 256 | 66 |
| `13` | `1cbc8` | 0 | 294 | 78 |
| `14` | `1cc28` | 0 | 294 | 74 |
| `39` | `1bd6e` | 0 | 1,112 | 136 |
| `3a` | `1be08` | 0 | 80 | 228 |
| `3b` | `1be64` | 0 | 1,152 | 26 |
| `3c` | `1bfce` | 0 | 960 | 26 |
| `3d` | `1c032` | 0 | 1,080 | 188 |
| `45` | `1c6e2` | 0 | 906 | 52 |
| 合计 | | 91 | 6,428 | 916 |

普通自然输入只覆盖后退动作 `11`。特殊 MAIN 的完整自然输入证据仍由角色组合范围补充，本表的私有样本不能替代自然特殊技验收。

34 条单初态轨迹分别以空 FX 池和满 FX 池执行 17 种动作流程。每条轨迹只初始化一次，再依次运行原版 `19c96` 时钟、当前 MAIN 和自身当前不可变动画记录的 `Duration/Flags` 刷新。合计 916 次真实时钟、916 次 MAIN、916 次记录刷新和 10 次明确的 Held／Pressed 输入变更。轨迹覆盖落地变体、三种瞬移位置、双子创建后持续控制与 C 退出、能量耗尽、第三子槽空／忙时的停顿，以及共享动作退出。没有回灌期望中间状态。

每次 CPU 阶段保留完整 64 KiB RAM 差异、父体 256 字节、三个子体 768 字节、FX 池 2,048 字节、游标及另外三个完整父体。自然后退样本包含 2 次 IRQ，影响 1 条 MAIN 记录；外部变化按 IRQ 的实际 before／after 链解释。MAIN 的 RNG 消耗、未解释变化和本控制下的外部音频清除调用均为 0。采集器保留音频清除事件及其链，不为 `7000..77ff` history RAM 设置豁免。

## 行为与状态所有权

`39` 直接使用构造器已带方向的 VX，随后累加 Gravity、VY 和 Y。下降时 contact 位 2／3 置 `AlternateVelocity` 最高 byte 的 bit0；按有符号坐标高 word 判断落地，夹到完整 Ground，并以该标记选择 `a1/a2`。

`3a` 只消费精确完成值 1。`AlternateVelocity` 最高 byte 的 0／1／2 分别选择 Left 加 `20/a0/120`，其他值直接使用 Left。仅替换 X 的高 word，并保留小数。

`3b` 在进入 `3c/a7` 并 reset 后独立尝试两个子槽，行为标识分别为 `136e6/1391e`。Attributes 使用父值加 `70/60`、mask `9fff`、OR `6000`；Ground 只复制高 word，Y 高 word 减 32，X 的 word 偏移按任意非零 Facing 反向。没有覆盖原版未写入的速度、Power、Pose、输入或物理 FE。

`3c` 每步从完整 Energy.long 扣 `4000`，包含低 word 借位与 long 回绕后的符号判断。非负时先写子一 Held／Pressed 的低 byte，再检查子一 Size；只有子一非空才写子二输入，子二没有 Size 过滤。能量为负、子一为空或 C 触发 `03/a8`。

`3d` 的精确完成值 1 优先进入站立。否则只有 frame2／ticks0 尝试初始化第三子体的行为、Size 和 Stage；子槽忙也继续将异 Team 父体的 `HitStopTicks` 写为 `ff`，不筛选 Size 或 Health。

`Fighter7MotionState` 复用 `ChuSpecialMotionState` 的完整 Energy、AlternateVelocity、Body 和 Children。Left 对应 `a010`。子输入复用现有 `Fighter4ChildExtra`；Direction、ContactFlags 和 DragonBank 等未写字段保持。物理 FE 仍由既有 DragonBank word 唯一拥有。

OtherFighters 恰好包含其余三个固定父体。自身 HitStopTicks 留在既有 actor owner。MAIN 复制 peer map 并返回修改，调用者负责一次原子合并；缺少／多余 peer、非法父体、有限表越界或分配诊断失败都返回原始状态，且不修改调用者 map。

## 测试顺序与结果

失败清单先于采集器、测试和生产实现保存。原版自然、dirty 和单初态夹具完成后冻结测试，再实际获得 `Fighter7MotionState`／`StepFighter7Motion` 缺失的编译 red，最后新增 `fighter7_motion.go`。

首次冻结的辅助 API 测试存在手工初态错误：Y 为 300、Ground 为 320，却沿用站立恢复断言。生产实现仍不存在时，主任务明确授权只将该测试初态改为 `Y=Ground`；期望值和 CPU 夹具未变。v1 全文、red、冻结摘要与 v2 的更正原因保存在 `fighter7-motion-v1-test.go.txt`、`fighter7-motion-v1-red.jsonl`、`fighter7-motion-v1-test-first.json` 和 `fighter7-motion-test-lineage.json`。v2 重新冻结并获得真实缺实现 red 后才编写生产代码。v1 的缺符号 red 未用作行为通过证据。

v2 测试 SHA-256 为 `cbea2f11e0efe11694c4042aca6722a1a6861edd9b2bc97ed6b52d3494613c51`，生产实现 SHA-256 为 `cfb8334a40adb24ceaf5232f09cf06a14fdc948c83f3e71308cf0cf746c26004`。实现首次执行即通过全部冻结断言，v2 冻结的 9 个文件摘要保持不变。

| 检查 | 结果 | 产物 |
| --- | --- | --- |
| 正式包原生竞态检查 | 通过，14.258 秒 | `testdata/fighter7-motion-native.jsonl` |
| 正式包 Node-WebAssembly | 通过，4.261 秒 | `testdata/fighter7-motion-wasm.jsonl` |
| 先行缺实现 red | 已核验为本范围缺符号 | `testdata/fighter7-motion-red.jsonl` |
| 子回调实现尚未就绪时的编译失败 | 单独保留，未作为通过结果 | `testdata/fighter7-motion-package-waiting-children.jsonl` |

此前的明确文件清单中间检查也通过；清单和日志使用 `fighter7-motion-intermediate-*` 前缀。正式包最终检查没有排除测试文件。来源、夹具、冻结链及日志摘要汇总在 `testdata/fighter7-motion-validation.json`。

## 复跑

在本工作树根目录执行；原版采集还需要 manifest 所绑定的现有本地 ROM、原版核心和观察核心。

```sh
export GOCACHE="$PWD/.local/go-cache"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
go test -race -json -run '^TestFighter7Motion' -count=1 ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm go test -json -run '^TestFighter7Motion' -count=1 \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter7_motion.py \
  --out .local/md-yuyuhakusho/fighter7-motion-recapture \
  --inputs docs/research/md-yuyuhakusho/source7-next-inputs.json \
  --reference games/yuyuhakusho/engine/testdata/fighter7-attacks-reference-frames.json \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter7-attacks-reference-manifest.json
```
