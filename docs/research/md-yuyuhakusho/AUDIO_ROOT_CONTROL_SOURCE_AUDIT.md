# Root control、fade 与清理范围的只读来源审计

当前 139 个比赛候选不包含会请求自动 duck 的 `7F`。从 Root 初始化／`04` 清零、平台跳过原菜单，并仅消费该候选闭包与已查音序语法的前提下，`38c2e` 的 control／duck 例程维持 idle。这个结论不包括 tempo、fade、pause，更不表示整个 header idle；`01/04/05` 仍是比赛代码实际可提出的控制请求。

本报告订正本轮早期「合法 fight PCM flag 可触发 duck」的概括。`7F` 是 dispatcher 可接收、具有真实状态效果的有效 descriptor，但它不在当前 139 个比赛候选中。私有 `7F` duck 向量可以验证更宽的可调用 profile，不能充作当前 fight 的请求来源。以下均为静态来源结论，没有执行原 CPU，也没有提供 production 状态图。

机器清单见 [AUDIO_ROOT_CONTROL_SOURCE_AUDIT.json](AUDIO_ROOT_CONTROL_SOURCE_AUDIT.json)，检查与来源绑定见 [VERIFICATION](AUDIO_ROOT_CONTROL_SOURCE_AUDIT_VERIFICATION.json)。固定 Japan ROM 的 SHA-256 为 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。

## E2、FF 与 F805

独立重走 22 个不同 music header 的 198 轨，以及 12 个 primary header 的 16 轨，共 12,775 条 command。ID、voice、source、quanta、command 和 argument 与既有静态 provenance 逐条相同。11 首比赛 BGM 加完整 12 个 primary 的子域为 115 轨、8,859 条 command；这 12 个 primary 是完整已恢复目录，不等于全部都已建立比赛 producer。

| 项目 | 已确认结论 | 保留边界 |
| --- | --- | --- |
| E2 → F803 | `38dc0` 的 E2 分派项指向 `38e74`，原指令复制一个 score-data byte 到 F803；全部已查 22 music／12 primary 路径中 E2 为 0 | 不排除未建立的动态 sequence 入口或外部赋值 |
| FF control 扩展 | FF 子表确实包含到 `393c4/393ec` 的入口；全部已查 grammar 中 FF 为 0 | 代码存在不等于已在比赛或菜单执行 |
| F805 | 117 个已保存静态文本中没有已确认的直接指令引用；完整 ROM 对齐 word 检索有两个未分类命中 | 不能据此将 F805 全局命名为 padding，间接／外部用途仍未排除 |
| fade period | `01` 写 F806=3、F804=40；`05` 写 F806=32、F804=50；后续 `38956` 直接写 F806=3 | 当前 fade 路径没有用 F805 作为 period，不应据相邻地址增加一个未经证明的独立计数 |

重走使用现有 pure score-data walker 的 `ARG_BYTES` 与 `walk_track` 两个 AST 节点，不执行它所在模块的 imports、native session 或命令入口。每轨上限仍为 200,000 条 command／1,000,000 quanta；有限／循环路径按原共享 loop／return words 结束，不把 argument 或 payload 中的 E2／FF 字节统计为命令。唯一目录内嵌队列边仍为 `AB → 7D`。

F805 的两个 raw word 位于 ROM `9d92e`、`2915ce`，未取得指令入口或保存列表支持。两个 bulk reset 会覆盖并清零 F805，这不为它建立 fade period 语义。

## F828～F82C 的真实 producer

| 字段 | 已确认写入来源 | 场景归类 |
| --- | --- | --- |
| F828 control mode | header 的 `38c4c` 写 2、`38c56` 写 0；FF03 的 `393d0` 写 1；FF04 的 `393f6` 在原值为 2 时写 `80`；bulk reset 清零 | FF producer 的比赛／菜单入口未建立；当前已查 grammar 不含 FF |
| F829／F82A delta | 自动 duck header 的 `38cbc/38cc2` 写默认 `0a/02`，restore 的 `38d8c/38d90` 清零；FF03 的 `393e2/393e6` 可以从 score data 取两个值 | 自动 duck 的普通 descriptor 入口只有闭包外的 `7F` 带请求位；FF 场景不明 |
| F82B duck phase | accepted PCM 分支 `37e56/37fc6/38076` 写 1；header 确认后 `38caa` 写 2，恢复时 `38cfc` 写 0；bulk reset 清零 | generic accepted PCM 路径真实存在，但当前 fight 候选没有所需新 descriptor flag5 |
| F82C choice bit | accepted dual A 的 `37f1e` 设 bit0，accepted B 的 `37fce` 清 bit0；bulk reset 清零 | 当前比赛候选包含 dual 类 descriptor；是否接受仍由仲裁决定，不能因 control idle 而把 choice bit 固定为零 |

新确认的 FF 来源不是任意字节命中。已有 Ghidra 列表明确给出 `38e3a` 的 indexed jump；固定 ROM 子表 `38e4a`、`38e4e` 的 BRA.W 分别指向 `393c4`、`393ec`。审计器核对这两段完整原字节模板，独立列出每条指令的地址、宽度和读取／写入角色；没有启动新 Ghidra 或执行目标。

FF03 的 argument 消费也有条件：Mode 非零时跳过两个 argument 后返回；Mode 为零时先写 1，只有原两个 delta 都为零才读取两个 argument，已有 delta 非零时直接返回。FF04 只在 Mode 为 2 时写 `80`。不能把 FF03 简化成无条件读取两个 delta。

全 ROM 的对齐 word 检索共保留 46 个 `F803/F805/F828..F82C` 命中。36 个由保存指令或上述明确分派表链接的 ROM 模板支持，余下 10 个保持「未分类 raw word」，没有提升为代码，也没有判定不可达。搜索还复核了 117 个静态文本中的 18,654 个去重指令地址；这不是全 ROM computed／dynamic call graph。

## 自动 duck 的闭包边界

直接从固定 ROM 重算 122 个 ordinary、F0 和 25 个 music DAC descriptor 的 flags，结果如下。

| descriptor 域 | 数量 | flag5 非零 |
| --- | ---: | --- |
| ordinary `06..7F` | 122 | 仅 `7F`，原 8 bytes 为 `07001c8014e0c000`，flags=`e0` |
| 特殊 F0 | 1 | 无，原 8 bytes 为 `0b80008014814000`，flags=`81` |
| music DAC | 25 | 无 |
| 当前 139 个比赛候选与 flag5 descriptor 交集 | 139 个候选 | 空；不含 `7F` |

三个 PCM request store 除新 descriptor flag5 外，还要求旧 PCM mode flag5 为零、当前 duck phase 不为 2；descriptor 可分派不等于本次仲裁已接受或已经执行 duck。当前闭包没有满足第一个条件的请求。

有限闭包论证的前提是：Root 从已清零的 profile 开始，平台不经过原菜单，只引入 139 个已审候选和闭合的 score 队列边，且没有额外 state 注入或未支持的 computed entry。`01/05` 不写 control／duck 字段，reset 将它们置零；已查 grammar 无 FF 请求；合法候选又不能触发三个 phase=1 store。Mode 与 duck phase 都为零时，原 `38c2e` 走 idle 返回，不会自行创建请求。

因此，该 profile 可以复用已证明的 control-idle 成本。这个结论不声称原游戏／菜单／任意 ROM 入口不可能 duck，也不要求把当前 fight 额外扩展为私有 `7F` 场景。fade、pause、priority、queue 与双槽 choice 的独立真实变化仍须保留。

## 01、05、04 与 music-reset 的范围和顺序

dispatcher 的五项原控制表明确映射 `01→38916`、`02→38808`、`03→388ac`、`04→38a2c`、`05→38928`。公共 dispatcher 在调用前已经将 selected 写为 `FF`；reset leaf 因 bulk clear 又清掉 selected，随后必须再次写 `FF`。

`01/05` 的顺序都是先写初始 remaining，再写 steps，最后 `CLR.B F840`，清整个 DAC music flag byte。它们保留 DAC 行其余参数／owner，也没有在这两个短函数中发布 PCM stop。fade 的 remaining 原值非零时只减一后返回；`1→0` 的同一调用不继续执行 level step。remaining 原值为零时才减 steps；`steps 1→0` 直接进入 `04`，不再做本次 level step。

| 清理函数 | CLR.L 次数 | 精确半开区间 | 声部边界 |
| --- | ---: | --- | --- |
| `04`，`38a3c` loop | `00e7 + 1 = 232` | `FFF800..<FFFBA0` | 64-byte header 加全部 18×48-byte voice |
| music-reset，`38a66` loop | `0087 + 1 = 136` | `FFF800..<FFFA20` | 同一 header 加前 10 个 music voice；保留 primary／secondary |

music voice 为 `FFF840..<FFFA20`，primary 6 行为 `FFFA20..<FFFB40`，secondary 2 行为 `FFFB40..<FFFBA0`。审计清单逐个保存 bulk store 地址及跨 owner 的 long 分组，例如第一笔同时覆盖 priority、tempo remaining／period 和 E2 signal；第二笔同时覆盖 fade 与 World pause；queue 又横跨后续 long。不能将这些分组重新解释成 tick 末一次无序 reset。

Scheduler `FFF560..FFF564`、Z80 PCM 区以及 TL cache `FFFBE0..<FFFC00` 均在两个 bulk clear 区间之外。`04` 不直接停止当前独立 PCM；它也没有先把整个 TL cache 清成零，后续 24 次 TL mute 才写入对应有效缓存／芯片项。

`04` 的静态展开顺序为：

1. YM bank0 register `27=00`。
2. 232 次 long clear。
3. selected=`FF`。
4. register `28` 的六次 keyoff，value 依次 `02/06/01/05/00/04`。
5. 24 次 TL=`7f`：channel 顺序 2、1、0；每个 channel 的四个 operator 按原次序，每次 bank0 后 bank1。
6. PSG `9f/bf/df/ff` 四次 mute，然后由该 leaf 返回。

music-reset 的 leaf 顺序为 YM `27=00`、读取并保存 priority、136 次 long clear、恢复 priority、selected=`FF`、返回。它本身没有完整 keyoff 或 PSG mute。外部 music-start 先调用 StopPrimary、StopSecondary，再调用此 reset；后续音乐初始化自己的声部 keyoff 不能倒算成 music-reset leaf 的内部写入。

以上顺序来自固定 ROM／保存列表和具名有限循环的算术展开，不是新观察的端口 ledger，也没有新增 master-cycle 或异步 IRQ 证明。

## 复跑与保留范围

```sh
python3 -B docs/research/md-yuyuhakusho/AUDIO_ROOT_CONTROL_SOURCE_AUDIT.py \
  --out docs/research/md-yuyuhakusho/AUDIO_ROOT_CONTROL_SOURCE_AUDIT.json
python3 -B docs/research/md-yuyuhakusho/AUDIO_ROOT_CONTROL_SOURCE_AUDIT_CHECK.py \
  --audit docs/research/md-yuyuhakusho/AUDIO_ROOT_CONTROL_SOURCE_AUDIT.json
```

失败清单和检查器先于审计器；真实缺失产物 red 保存在 [RED.json](AUDIO_ROOT_CONTROL_SOURCE_AUDIT_RED.json)。检查器随后复核 129 个来源 SHA、语法域、descriptor 交集、明确 producer、清理边界及 mute 顺序。重复生成的清单必须字节相同。

本轮没有加载 native core、运行 Go、安装工具、修改冻结 leaf／score／engine／旧证明或总索引。`-B` 避免 Python cache；未产生需清理的构建、profile 或原采临时产物。仅保留新审计文档、脚本、red、机器清单和来源校验。
