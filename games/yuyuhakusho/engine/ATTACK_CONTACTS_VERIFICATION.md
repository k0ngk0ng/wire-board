# 攻击命中／防御：实现前验证范围

本阶段研究完整 `0x1f21e..0x1f522` 攻击对受击形状循环，以及实际调用的防御、受伤、反应表、特效和 KO 辅助函数。先列出失败场景、保留原 CPU 完整输入／输出、编写测试并观察缺实现失败，再写独立 Go 规则。原版调用和隔离矩阵不等于完整对局。

可能失败的场景：把有序双循环改成无序去重；漏掉同队、地面高 word、单向目标掩码及动态位号模 32；word 坐标差取负或 byte 宽高相加失去回绕；将接边视为不命中；防御掩码／攻击元数据位错误；错误覆盖同一对象的攻击／受击字段；动画计时、朝向、停顿与输入保留顺序错误；只恢复扣血而遗漏硬直、体力、眩晕、反击、反应表和 KO；在 HP 等于零时提前判 KO；把有符号比较改成饱和减法；把空中判定所用的高 word 与完整定点数混淆；防御边界与受伤边界的不同误用；特效分配失败的游标、旧槽字段或随机数未保留；把间接分派表后的数据解码为代码；只比较选取字段，未核对完整人物、形状、特效池和全局副作用；被动观察改变整帧基线；从每步原中间态重新初始化来冒充连续对局；把测试成功扩大为角色／玩法全部完成。

所有采样、缓存、构建和分析临时文件仅写入当前 worktree。最终规则不得读取 ROM 或依赖模拟器运行。

## 已恢复范围

`ResolveAttackContacts` 按原顺序扫描攻击和受击记录，处理队伍、地面高 word、目标掩码和接触矩形；随后执行防御／受伤分支、全部 11 个反应构造、命中特效、连击数字特效以及 KO 声音／慢动作请求。完整角色运动、倒地起身、KO 后续流程和特效生命周期仍不在本组件中。

| 原版证据 | 完整调用 | 接触 | 防御 | 受伤 | KO 请求 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 固定 3,600 帧自然输入 | 2,068 | 11 | 0 | 11 | 0 |
| 原 CPU 隔离矩阵 | 915 | 895 | 288 | 607 | 66 |

自然输入只覆盖反应索引 0、1、4、5、9；隔离矩阵覆盖 0..10。自然记录未验证防御或 KO，这两种路径由明确替换私有输入的原 CPU 调用验证。所有 3,600 帧完整 RAM／画面与固定基线一致，两个证据集合均无未知 RAM 写入。隔离用例包括四人物 16×16 记录、同对象多形状、改队后续顺序、池满／单槽可用、动态位号、地面小数、坐标／尺寸回绕、生命／硬直 word 边界及连击计数 byte 回绕。

测试对每次调用比较参与人物完整 256 bytes、两个各 256 bytes 的形状缓冲、完整 2,048 bytes 特效池、缓冲偏移、分配游标、待处理声音、`FFF558`、`A0A4` 和场地边界。Go 规则只接收语义状态，没有读取原版输出或逐步注入期望中间态。当前证据是完整组件调用，未宣称连续独立人物更新或完整对局。

## 已确认的边界

- 生命先做 word 减法，结果小于零才请求 KO；生命等于零仍走非 KO 路径。防御削血会将负值清为零，不发 KO 请求。
- 碰撞过滤比较地面高 word。攻击方是否在空中也比较地面／Y 的高 word；不将小数不同自行判为悬空。
- 反应 6、7 会改变受击者队伍，影响后续记录；原循环顺序不能重排。
- 攻击强度低两位为 3 时保留原速度／抖动，不补出第四档数值。
- 特效中心使用两个坐标高 word 相加回绕后算术右移，保留旧槽中的坐标低 word。
- 连击数字使用共用分配算法，行为 ID 为 `0x14930`，另写 `+3c` 参数；不清空旧槽未写字段。普通命中特效仍使用 `0x149ac`。
- 反应表 `0x1f534..0x1f548` 的 11 项由相对偏移独立定位；当前动画目录中实际攻击元数据的反应值也仅为 0..10。表后的指令未作为更多表项。KO 声音表 `0x1fa8a..0x1fa94` 为 11 个角色来源 ID 的只读数据。

静态指令见 [主循环](../../../docs/research/md-yuyuhakusho/static/collision-consumers.txt)、[反应与辅助函数](../../../docs/research/md-yuyuhakusho/static/attack-contacts-helpers.txt)、[数字特效分配器](../../../docs/research/md-yuyuhakusho/static/attack-contacts-allocator.txt)。REA 当前不支持该原始 68000 目标；静态分析直接使用已安装的 Ghidra，实际 MC68000 行为由固定原内核验证。

## 验证产物与复跑

来源摘要、夹具摘要、测试结果和限制见 [验证报告](testdata/attack-contacts-validation.json)。[原生竞态日志](testdata/attack-contacts-native.jsonl)、[Node/Wasm 日志](testdata/attack-contacts-wasm.jsonl) 和 [实现前真实失败](testdata/attack-contacts-red-phase.jsonl) 一并保留。红阶段因本组件类型／函数尚不存在而失败；同一共享工作区的投技测试当时也尚缺类型，原日志保留了该事实。

从 worktree 根目录运行原版采样：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_attack_contacts.py --out .local/md-yuyuhakusho/attack-contacts/capture
```

运行独立规则验证：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" go test -count=1 -race -json ./games/yuyuhakusho/engine -run '^TestAttackContacts'
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" GOOS=js GOARCH=wasm go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine -run '^TestAttackContacts'
```

原版采样使用的 ROM、加观察钩子的内核和参考前端摘要保留在两个夹具清单。夹具包含有声明范围的原始输入／输出；最终游戏规则不依赖 ROM、原 CPU 或原始内存。没有接入游戏页面、发布素材或部署应用。
