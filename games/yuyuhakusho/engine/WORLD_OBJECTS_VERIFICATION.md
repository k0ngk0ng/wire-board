# 世界对象统一存储：实现前验证范围

世界层只保存已恢复规则实际拥有的具名字段，不保存整幅 RAM，也不在每个 tick 用字节序列化／反序列化拼接组件。相同原字段只有一个存储：`+10` 的速度／状态相位、`+48` 的 long／高 word owner 或寿命／高 byte 尾迹相位，以及 `+5a` 的时长／延迟均由适配器解释，不能维护互相失步的副本。

失败场景：清除整个结构而丢失实际前缀外字段；误将零清理请求等同于删除；`+71..75` 低字节与 `+c0..c2` 扩展输入一并清除；16-byte 边界切过命令数组时错误清全部；清理逻辑对象 ID；父人物回写子槽时覆盖其动画／图形缓存／战斗字段；特效分配器回写时覆盖生命周期附加字段；碰撞回写时清除未拥有的控制通道或基础队伍；高 word／byte 别名丢失低位；ID 不一致时部分修改；map／slice 调用者状态被修改；测试按当前输出回灌原期望。

先从已有静态证据与原 CPU `0x3556` 完整返回保存自然／私有输入边界，再编写测试并保留真实缺实现失败，最后实现具名字段和直接类型适配。清槽验证完整 256 bytes；测试适配器保留未知字节并按原清理请求清除它们，生产结构只清已拥有字段。前缀 0 表示没有清理，实际删除的默认大小由调用者已验证的删除规则产生。

本模块不是对象调度器。小于 256-byte 的管理／呈现对象只应由对应组件使用其已拥有字段；不能把未知的跨槽 RAM 别名当作该对象新增状态。完整对象迭代、世界级共享状态和精灵提交仍由世界／呈现组合验证。

追加前 64 bytes 呈现字段时，先扩原 CPU 清槽适配器与测试。新增 `Aux0A/Aux0E` 保存两屏幕 word 后的独立 word，`Aux18` 保存 `+18` 完整 long。覆盖层计数只替换横速高 word；Timer／Data 分别使用 `Aux18` 高／低 word；天空 Phase 与 Sequence 分别使用 `Aux48` 和 `AnimationSet` 完整 long；影子 Cursor 只替换 `AnimationSet` 高 word。失败边界包括丢失横速小数、错误将天空 Phase 当动画集合、修改天空未拥有的 `+5a`／Completed／SkipGraphics，或把 64-byte 覆盖层后续槽字节写回本体。

## API 与状态所有权

`WorldObject` 由逻辑 `ObjectID`、`MatchObjectHeader`、`FighterBodyState`、`FighterControls`、完整能量以及具名图形、战斗、输入／AI／HUD 字段组成。角色 ID 与通用 FX 类型共用 `Tag`，必须结合对象种类解释。动画／倒计时序列共用 `AnimationSet`；该数值是已解码库的来源标识，不执行原代码或访问 ROM。

`As…` 直接构造已有组件的类型视图；`Merge…` 只合并该组件实际拥有的字段。返回的组件状态中世界缓冲／声音／池等字段由调用者另外填写和串传，不能因适配器默认零值重置世界。带 ID 的合并拒绝跨槽身份，失败时返回原对象。

| API | 写入范围 |
| --- | --- |
| `AsChuFighter`／`MergeChuFighter` | 人物本体，保留 header 回调／阶段和未由人物规则拥有的图形缓存 |
| `AsChuChild`／`MergeChuChild` | 父构造器拥有的子槽子集；不覆盖子槽动画时钟、图形缓存、控制或攻击标志 |
| `AsAnimation`／`MergeAnimation` | 已恢复的动画提交字段 |
| `AsCollisionActor`／`MergeCollisionActor` | 碰撞消费字段，保留输入扩展位、Pressed／Changed 与基础队伍 |
| `AsEffectSlot`／`MergeEffectSlot` | 分配器及通用初始化共同字段，不清生命周期附加状态 |
| `AsChuProjectile`／`MergeChuProjectile` | 弹体规则字段，尾迹只替换 `Aux48` 最高 byte |
| `AsChuAttachment`／`MergeChuAttachment` | 附件字段，来源偏移只替换横／纵速度的高 word |
| `AsEffectMotion`／`MergeEffectMotion` | 通用 FX，跟随 owner 只替换 `Aux48` 高 word |
| `AsAttachedEffect`／`MergeAttachedEffect` | 数字／眩晕 FX，owner／寿命共享 `Aux48` 高 word |
| `AsFighterInitialization`／`MergeFighterInitialization` | 完整人物初始化视图，header 保留 |
| `AsReactionExtra`／`MergeReactionExtra` | owner 高 word、Y 偏移和 `+5a` 的同一存储 |

`WorldObjects` 是稀疏逻辑槽 map。`LookupWorldObject` 为缺省槽返回携带逻辑 ID 的零对象，拒绝不一致的存储 ID 和对象池外／未对齐 ID。`BuildChuActorFighter`／`MergeWorldChuActor` 读取或合并人物及三个子槽，合并时复制 map；`ChildAttackFlags` 是 AI 的只读视图，不回写子槽。`BuildWorldEffectPool`／`MergeWorldEffectPool` 只读取／合并 16 个 FX 槽的分配器共同字段，游标仍由世界单独持有。

删除结果的 `ClearedBytes` 须先用于 `ApplyObjectClearPrefix`，再合并回调返回字段；这样本组件未拥有的其他已恢复字段也按原前缀清除。前缀 0 是无清理请求，不是原 `Size==0` 的默认 64-byte 删除。实际 `3556` 如何由 Size 产生请求已有各回调及本矩阵验证，不能把错误大小扩展为任意 RAM 模拟。

## 已验证结果

原 CPU `3556` 的 61 次自然完整调用（7 次 64 bytes、54 次 128 bytes）及 288 个隔离调用完整一致；所有 16-byte 前缀 16～256 均覆盖，另包含原 Size 0 的默认 64-byte 和非整倍数 Size 向下取整。固定 3,600 帧 RAM／视频基线不变，没有未完成调用、范围外写入或被排除自然样本。静态指令来源沿用已保存的直接 Ghidra 证据，见 [world-objects-clear.txt](../../../docs/research/md-yuyuhakusho/static/world-objects-clear.txt)。

5 项顶层测试在原生竞态与 Node/Wasm 均通过。除全部清槽字节外，测试还检查：

- 稠密不同输入与已有独立原字节解码器得到相同动画、弹体、附件、碰撞和两类 FX 视图。
- 每个视图／合并往返保留未拥有字段；高 byte／高 word 别名保留其余低位。
- 人物及三个子槽和 FX 池合并保持原 map 不变，后置 ID 错误没有部分写入。
- 实际 `AllocateEffect` 结果只改变其拥有字段，已有生命周期字段不被分配回写抹除。
- 915 个原攻击边界完整调用由现有 Go `ResolveAttackContacts` 计算，再合并进统一对象；人物完整 256 bytes 与原 CPU 结果一致。预期原输出没有送入生产适配器。

测试文件与原版夹具先于生产代码建立，真实聚焦构建实际报告缺少 `WorldObject`／`ApplyObjectClearPrefix`，来源与红／绿／Wasm 日志见[验证记录](testdata/world-objects-validation.json)。这证明统一存储及适配边界，不证明世界循环或完整对局。

## 复跑

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_world_objects.py \
  --out .local/md-yuyuhakusho/world-objects-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestWorldObjects' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestWorldObjects' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

聚焦证据运行采用已提交源码／测试文件加本模块的明确文件列表，避免其他代理并发红阶段阻断构建；父世界阶段仍须执行完整包回归。没有提交、发布或修改其他组件。

## 前 64 bytes 呈现适配扩展

在原统一对象验证后，按世界组合所需增加 `Aux0A/Aux0E` 两个独立 word 与 `Aux18` long，使前 64 bytes 的已恢复字段各有唯一存储。原 CPU 清槽 decoder／encoder 与新先行测试先修改；实际构建因这些字段及适配器尚不存在而失败，之后才增加生产字段和方法。初始 5 项测试的红／绿证据原样保留，扩展另有独立红阶段与当前 8 项测试的原生竞态／Wasm 结果。

`AsMatchOverlay`／`MergeMatchOverlay` 只替换 `Body.VelocityX` 高 word 作为 Counter，保留其低 word；Timer／Data 共同组成 `Aux18` 的高／低 word。屏幕两个 word 后的 `Aux0A/Aux0E` 不被覆盖层误改。`AsArenaSky`／`MergeArenaSky` 保留实际 `+0a` 分数，Phase 是完整 `Aux48`，Sequence 是完整 `AnimationSet`，并只更新实际拥有的三个时钟字节，不清 `+5a`、Completed 或 SkipGraphics。`AsMatchShadow`／`MergeMatchShadow` 将 Cursor 作为 `AnimationSet` 高 word，保留低 word；Registration 仍是调用者的世界状态。

新增黄金行从已验证原 CPU 夹具确定性派生，不是再次执行目标：8 类覆盖层／影子共有 484 行，天空 sun／moon／spark 共 59 行，覆盖阶段、暂停／慢动作、绘制、静默返回、删除及新槽建立的代表组合。每行实际运行现有独立规则，再通过 canonical 适配器合并，完整原对象区与 CPU 输出一致；没有给生产适配器回灌期望输出。派生清单绑定原压缩夹具、原 manifest 和派生脚本 SHA；重建命令为：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/derive_world_object_views.py \
  --out .local/md-yuyuhakusho/world-objects-views-replay
```

代表集不把其他 18,354 个覆盖层／4,456 个场景边界的重复规则验收计作本模块新 CPU 执行，也不代表世界循环或像素呈现完成。所有当前来源与保留的前后测试阶段均在同一验证记录中。

## 覆盖层 API 同步

后续覆盖层恢复扩展了共享模式／慢动作状态，并将原对象采样窗口扩为 `8100..83ff`。统一对象的生产字段与适配器无需改变；验证代码改从 `MatchOverlayState.ModeFlag/SlowFlag` 传入共享状态，按 `8100` 基址读取 768 bytes，并补充 Round／Blink 只读视图。派生脚本明确限定既有 8 类覆盖层，代表集仍为 484 行覆盖层／影子和 59 行天空。

更新前实际构建报告已移除的 `MatchOverlayEnvironment.SlowFlag` 字段。修复后 8 项统一对象测试的原生竞态检查与 Wasm 均通过，分别为 2.596 秒和 0.523 秒。兼容性日志独立保存为 `world-objects-overlay-compat-{native,wasm}.jsonl`，更新后的样本 manifest 重新绑定扩展源夹具与脚本。原先实现前红灯和历史来源摘要继续保留；新增兼容性记录说明当前版本，不改写过去的执行证据。
