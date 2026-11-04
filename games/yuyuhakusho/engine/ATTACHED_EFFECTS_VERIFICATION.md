# 连击数字与受击附着特效：实现前验证范围

恢复 `0x14930` 连击数字及 `0x14ec6` 受击／眩晕附着特效的完整规则阶段，直至 `0x1580` 绘制请求或 `0x3556` 清理完成。先保留原 CPU 完整输入／输出，再编写测试并观察缺实现失败，最后加入独立 Go 规则。

可能失败的场景：把分配初始化等同于完整生命周期；错误解释 `+3c` 参数、父指针、偏移或 phase 字段；依赖错误的父状态／对象编号；暂停仍推进寿命或错误停止绘制；缺少父对象消失／受击结束后的清理；将 word／byte 写入改为整个 long；提前判断删除或遗漏最后一帧；清理时把整池或整块对象错误归零；同池多个对象相互覆盖；误把精灵提交当规则阶段；忽略原对象范围外副作用；仅依赖重新编码自洽性而不比较原 CPU；每步回灌原版输出冒充生命周期；缓存或临时文件写到 worktree 外。

自然采样与同输入未观察原内核的完整 RAM／视频基线比较；隔离输入须显式声明。测试覆盖完整效果对象、父对象、实际写入的池／全局范围和生命周期回收。连续轨迹只提供外部父／世界输入，不注入预期效果中间态。绘制编码、世界完整调度和完整对局仍单独验收。

## 已确认规则

接口为 `StepAttachedEffect(state, parent, world, library)`，返回下一状态与 `Draw/ClearedBytes`。零绘制且零清理表示原回调的静默 RTS，不能据此补画效果。规则不写 MD DMA／SAT 缓冲；绘制请求供独立呈现层消费。原指令见[特效核心](../../../docs/research/md-yuyuhakusho/static/attached-effects-core.txt)。

连击数字的 `+48.w` 是寿命计数。初始化保留旧计数与 `+3c` 图形参数，设置字体集合、跳过图块传输、属性和深度；正常更新先递增 word，结果恰好为 32 才清理。其他值继续运行，包含 word 回绕；不能改为“大于等于 32”。未删除时只把 Y 的高 word 减一，保留定点小数，再执行投影。暂停／慢动作冻结计数和位置，但仍请求绘制。

受击附着特效的 `+48.w` 是父对象 ID；`+5a.b` 是 0／21／42 等初始错开延迟，不是普通动画帧时长。阶段零按类型 0～2 绑定三种倒计时序列后立即返回，首帧不画、不减延迟。之后若暂停／慢动作则直接请求绘制；正常路径先检查父动作 5、帧索引 0、帧计时 0 的重启门槛并清理，再检查延迟。延迟非零时减一并静默返回，不推进序列或位置。

延迟结束后推进独立倒计时序列，复制父 X／Y 高 word 并保留本效果的小数；父 `+84.w` 按 signed word 小于 1 时清理。正常持续路径给 `+10.w` 相位加 16 后 `&03ff`，读取原表的 signed byte，分别算术右移 2／3 位并加到 X／Y 高 word，再装入该父角色的垂直偏移并投影。相位只占 `EffectSlot.VelocityX` 的高 word，低 word 保留。

只读数据包括三条 `0x15082/0x15088/0x1508e` 序列（每条两帧、时长 8、零时长终止），`0x34000/0x34200` 起每隔两个 byte 读取的 1,024 项轨道数据，以及 `0x1506c` 的 11 个角色偏移。数据是规则输入，独立实现不读取 ROM。字体图形集合为 `0x27f618`，初始化不重写旧图块基址；`SkipTiles=1` 是原呈现路径的重要条件。

删除使用通用 `0x3556`：size 为零清 64 bytes，否则清 `size>>4` 个 16-byte 块。本阶段保留单槽 128 bytes，验证 size 0／16／31／64／128 与未清后缀；拒绝会逃出该存储范围的清理，不声称模拟原版损坏 size 导致的地址别名。返回 `ClearedBytes` 供世界池同时清理本规则未拥有的字段。

## 已验证结果

- 548 次自然规则阶段：连击数字 32、受击附着 516，共 478 次绘制、66 次静默返回、4 次真实回收。记录均来自原基线输入；另一个攻击脉冲输入没有触发这两类效果，不扩大自然逻辑覆盖。
- 两个明确输入各 3,600 帧，观察内核的完整 RAM／视频均与同输入未观察原内核一致。
- 4,294 个原 CPU 隔离用例：连击数字 1,440、附着 2,854，覆盖三种返回、初始化／非零阶段、延迟与父重启门槛顺序、signed 眩晕值、全部 1,024 相位、11 个角色偏移、计数与坐标回绕及部分清理；包含 264 次回收。
- 12 条单初态生命周期共 538 步，全部到达回收。连击数字 6 条共 226 步，附着 6 条共 312 步。只变化声明的父／世界输入，不恢复效果中间态。
- 每次比较完整 128-byte 槽、2,048-byte 池、256-byte 父对象、池游标与所读世界状态。隔离执行另外检查整个 RAM 的范围外写入，均为零；本规则不分配槽、修改其他池槽或产生声音／形状。
- 原生竞态与 Node/Wasm 均通过 3 项顶层测试。测试和真实缺实现失败先于 Go 实现；不可变序列、无效类型／父偏移／库及非法清理范围的原子失败也已验证。

取证阶段修正了连击数字生命周期中“固定观察父地址与声明外部更新地址不一致”的脚本问题，并重新采集全部原版记录与复跑；没有屏蔽父对象差异。数字本身不依赖父对象，该对象只用于完整非干扰比较。当前来源、夹具及红／原生／Wasm 日志在[验证记录](testdata/attached-effects-validation.json)。

## 复跑

从 worktree 根目录采集：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_attached_effects.py \
  --out .local/md-yuyuhakusho/attached-effects-replay
```

完整引擎就绪后使用 `go test -race -count=1 -run '^TestAttachedEffects' ./games/yuyuhakusho/engine`。并发其他组件尚在红阶段时，从 `games/yuyuhakusho/engine` 可独立复跑同依赖范围：

```sh
ATTACHED_EFFECT_ROOT=$(cd ../../.. && pwd)
mkdir -p "$ATTACHED_EFFECT_ROOT/.local/md-yuyuhakusho/tmp"
GOCACHE="$ATTACHED_EFFECT_ROOT/.local/go-cache" \
GOMODCACHE="$ATTACHED_EFFECT_ROOT/.local/go-mod" \
GOTMPDIR="$ATTACHED_EFFECT_ROOT/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json attached_effects.go effects.go motion.go attached_effects_test.go
GOOS=js GOARCH=wasm GOCACHE="$ATTACHED_EFFECT_ROOT/.local/go-cache" \
GOMODCACHE="$ATTACHED_EFFECT_ROOT/.local/go-mod" \
GOTMPDIR="$ATTACHED_EFFECT_ROOT/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  attached_effects.go effects.go motion.go attached_effects_test.go
```

仍需世界池调度、完整对象清理合并、字体资源及独立呈现集成；本阶段不作为完整对局或平台 E2E 证明。
