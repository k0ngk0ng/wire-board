# 落地／起跳特效池：实现前失败场景

本清单先于本批采样器、夹具、测试和 Go 实现。恢复范围为 `0x1fa96` 经 `0x1fab4` 的分配器，以及 `0x1fce8..0x1fd80` 的成对尘土初始化；不包括 `0x149ac` 的后续效果更新与绘制。

- 把 16 个槽都扫描一遍，而原版每次最多检查 15 个槽。
- 先检查当前游标槽，再推进游标；原版顺序相反。
- 满池或失败时不保留已推进的游标；回绕时遗漏 word 截断或 `0x7ff` 掩码。
- 以更新入口是否为零判断空槽，而原版依据 `+4.w` 槽步长。
- 分配时清空整个槽，丢失没有被本阶段改写的残留字段。
- 两个尘土对象使用一次原子分配，遗漏一侧失败／另一侧成功的实际行为。
- `MOVE.B` 对属性 word 的高字节写入被误当低字节写入，或错误保留掩码应清除的低位。
- 世界坐标整数 word 的 ±4 与 byte 层级的 -4 丢失回绕或误改小数。
- 复制源人物字段不全，或用模板替换深度、层级、Y、速度和效果类型。
- 原函数未返回、嵌套分配配错边界、采样改变基线，或掩盖未知对象写入。
- 用隔离改写私有模拟器状态的样本冒充自然完整对局。
- 向独立规则传入不满足原池步长的游标，静默取整后得到看似合法的分配。

自然夹具来自已有 3,600 帧冷启动输入，并核对观察前后整帧 RAM／视频摘要。隔离夹具每次恢复同一原版检查点，明确设置池占用和源人物输入，由实际 68000 函数产生期望结果。测试必须先于 Go 规则实现，并比较完整 0x800-byte 池：只把独立语义字段写回原始前态副本，其余字节不得变化。

## 结果与接口

已先采集原版数据并写测试；`effects.go` 尚不存在时，目标测试以 `undefined: EffectPool / EffectSlot / AllocateEffect / SpawnMovementDust` 编译失败。随后实现 Go 独立语义，原生 `TestEffect*` 通过。竞态与 Wasm 由父任务统一验收，本报告不冒称这两项已运行。

| 范围 | 实测结果 |
| --- | --- |
| 自然成对初始化 | 17 个完整原调用，整段 3,600 帧 RAM／视频与既有无观察器基线相同 |
| 原 CPU 隔离分配 | 54 个完整调用 |
| 原 CPU 隔离成对初始化 | 54 个完整调用 |
| 完整池对照 | 125 × 0x800 = 256,000 bytes，全部一致 |
| 成对结果 | 双成功 35 次、双失败 9 次、仅右成功 9 次、仅左成功 18 次 |
| 未完成调用／观察异常 | 0／0 |

隔离用例覆盖空池、满池、仅当前／下一／最后扫描槽空闲、交替占用，游标 `0 / 0x780 / 0xff80`，非零步长 `0x80 / 0x100 / 0xffff / 1`，残留槽数据、属性高 byte／word 掩码及坐标／层级回绕。原版把任何非零步长当成占用，不检查更新入口是否为零；即使当前槽空闲，每次也只检查推进后的 15 个槽。

接口为 `AllocateEffect(EffectPool) (EffectPool, int, error)` 和 `SpawnMovementDust(EffectPool, EffectSource) (EffectPool, [2]int, error)`，未分配索引为 -1。初始游标的高位按原版第一步掩码处理；低 7 位非零违反此语义池的槽对齐条件，接口明确拒绝且不修改状态，不假装复刻损坏的内存布局。

`EffectSlot` 只命名本阶段拥有的字段；测试适配器将这些字段写回原始前态的完整槽副本，对全部输出字节比较，确认本阶段未清空其他字段。`Behavior` 的 `0x149ac` 是恢复的分派标识，不是运行时调用原机器码的地址。未来的效果生命周期和呈现仍需独立恢复。

属性最终为 `(source.Attributes & 0x9800) | 0x2000`。原版先复制高字节、再 AND 整个 word、最后对高 byte OR `0x20`；不能误写为低 byte 的 `0x20`。位置 X 的整数 word 分别加 -4／+4，小数保留；Y、深度复制；速度为 -1／+1；层级 byte 减 4；效果类型为 `0x10`。

## 复跑

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_effect_pool.py --out .local/md-yuyuhakusho/effects-capture
mkdir -p .local/md-yuyuhakusho/effects-tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/effects-tmp" \
go test ./games/yuyuhakusho/engine -run '^TestEffect' -count=1 -v
```

原内核、ROM 和被动 PC 过滤器沿用研究目录固定版本。夹具为 `testdata/effects-oracle.json.gz`，来源与解压前后摘要在同名前缀的 manifest。隔离运行会修改私有模拟器输入，但每次都先恢复同一检查点，不把这些用例计作自然对局。
