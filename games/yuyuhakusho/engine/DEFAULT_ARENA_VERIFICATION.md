# 默认场景运行规则：实现前验证范围

恢复实际默认场景回调 `0x20918`、其相机／滚动 helper，以及实际创建的 `0x150ca/0x1525e` 色盘对象和世界调用的 `0x233c` 渐变。静态素材不能代替相机、样本消费、色盘推进或生命周期完成信号。

可能失败的场景：场景编号和回调绑定推测错误；暂停／慢动作时仍推进；初始化与更新落入关系错误；相机消费本轮而非上一轮人物样本；word 求和回绕后错误作有符号除法；样本 byte offset 与样本数量混用；相机限界、平滑、世界边界和清计数顺序错误；场景滚动累积的定点高／低 word 混淆；在反馈偏移非零时错误推进波纹；有符号表索引／算术移位错误；未保留逐行滚动及色盘依赖；淡入淡出依赖硬编码时长而非实际状态；覆盖未写字段；独立函数的缓冲写入或其他对象被遗漏；只把旧输出灌回新输入就声称连续更新。

先保存完整原版自然调用、明确私有输入的原 CPU 隔离边界和单初态流，再编写测试并保留缺实现失败，最后加入具名独立规则。全部写入、缓存和临时工程留在当前 worktree。验证边界若排除 VDP 硬件编码或绘制，须明确返回语义请求，不能声称完整视频呈现。

## 已恢复接口与顺序

`StepDefaultArenaCamera` 恢复 `21556`。它读取上一轮人物的两组最多 16 个定点坐标高 word，先按 word 回绕求和，再执行无符号平均。X 目标经 word 减 160 后限至 744～984，Y 经减 192 和算术右移两位后限至 0～32。当前相机分别用差值的 1／8、1／4 平滑，重写左右界并清 `A00E`；没有样本时全部保持。不能用宽整数／有符号平均替换原运算。

`StepDefaultArena` 恢复 `20918` 全部返回路径，包含 `21690` 反馈复制。暂停 bit7 或任意非零慢动作标志会直接返回，连初始化和样本消费也跳过。正常阶段 0 设置滚动模式 3、清两组漂移／波相后进入阶段 2；后续相机、垂直偏移、视差、水波与尾部 56 行查表均按原顺序生成。反馈非零时漂移不推进，并将 224 行加反馈量复制到独立输出行。

`StepArenaSky` 恢复 `150ca`、`1525e` 与后者实际生成的 `15330`，终点为 `1580` 绘制前或 `3556` 完成清槽。Sun 在暂停前检查，暂停阶段 0 不初始化；Moon／Spark 先初始化再检查暂停。Sun 按原四段目标渐变首 16 个场景色，仅在 `F501==0` 和特定低 20 位相位为零时推进颜色。Moon 在相位 384～640、无反馈时先消耗 RNG，再检查低 byte 和下一槽占用；占用也不能跳过已发生的随机调用。Spark 初始化消耗两次共享 RNG，按真实 17 项倒计时序列推进至删除，返回实际清理前缀，调用者须同步清理未纳入此类型的槽字段。

`AdvancePaletteFade` 是 `2300` 完整函数，`ApplyPaletteFade` 是 `233c` 含尾跳复制的完整函数。前者更新 mode／step、清完成 bit7；后者按 `F501.bit0` 选来源、bit1 跳过颜色输出，使用 `F502/F506` 的首地址 byte。由于原版为 big-endian，这两个 byte 是 mode／target word 的高 byte，不能直接转换为 `uint8(word)`。第一轮自然与隔离对照发现了这个错误，修正后又补齐全部八个插值行的 dirty-color 矩阵，最终完整通过。各色 odd／保留位和 word 旋转仍保留原语义。世界帧调用者另负责 `F500.bit0` 的颜色更新门槛，不得将两接口颠倒。

## 已验证结果

- 14,772 次自然完整规则调用，原输入全部 3,600 帧 RAM／视频与未观察基线相同。自然种类为 camera／arena／sun／moon／palette progress／colors，未观察到 Spark 生成，不把隔离生成冒充自然触发。
- 4,456 个原 CPU 边界中，4,450 个完整语义输出一致；6 个刻意损坏的持久 Moon 相位会读已解码波形外相邻 ROM，原输出作为 `reference-only` 完整保留，独立 API 明确原子拒绝。正常 Moon 每次更新后相位限制在 10-bit 周期内。
- 六条单初态流共 1,726 次调用，没有期望中间态回灌。含 180 轮 arena→sun→moon→实际存在的 spark→fade→colors 的共享状态组合，真实生成一次并回收一次 Spark；另外单独 Spark 生命周期也到达删除。外部输入只有明确声明的人物坐标样本，时钟、相位、颜色、头部、随机种子和删除由规则自行推进。
- 全部未知字节保留比较：128-byte 场景窗口、384-byte 三天空对象区、相机／128-byte 样本、HUD 前缀、两组各 256 行滚动、两组各 224 行反馈、垂直寄存器窗口、三组完整 64 色、fade 字节及共享 RNG。没有声音队列例外或输出字段屏蔽。
- 三项顶层测试的指定源码聚焦原生竞态及 Node/Wasm 均通过；父阶段还须运行整包集成。真实缺 `DefaultArenaCamera/ArenaPaletteState` 编译失败先于生产实现，日志及源码摘要保留于[验证记录](testdata/default-arena-validation.json)。

## 世界存储与呈现边界

`DefaultArenaState.RowsA/B` 分别对应 `E800..E9FF`、`EA00..EBFF`。`DefaultArenaView.HUDPrefix` 是当前 `E600..E7FF` 的只读 256-word 视图；负反馈行选择确实可以从这个 HUD 前缀开始。旧 HUD 完整取证窗口的后半部与 RowsA 别名，世界层须用同一存储或显式适配，不创建独立且过期的两份值。支持的反馈行范围为该前缀及两滚动 bank 的实际可用范围；损坏相机导致的任意地址读取不作为规则模拟。

SourceA／B 数值保留原传输来源身份，独立规则按具名行数组计算，不解引用原地址。`PanA/B` 是反馈后的行数据，实际 DMA／VDP 编码不在这里执行。Sun／Moon／Spark 的图形集合 `da16a`、图块源 `24b620` 也只是素材身份，不执行原程序。

色盘三 bank 和 `F500/F501` 必须在 match manager、动态对象、`2300` 和 `233c` 之间串传，不能每个对象从初始素材重置。Sun 只写 `Primary[0:16]`；Actor 换色、其他场景和光栅可见行仍属于各自的真实输入／呈现层。独立规则库 [arena-runtime-rules.json](../data/arena-runtime-rules.json) 保存已验证波形、视差行表、渐变表、四目标色盘与 Spark 序列，不包含图像、ROM 程序或模拟器。

本阶段没有恢复其余七场景、整个世界调度、精灵提交、完整光栅合成或平台渲染。自然 3,600 帧也未出现 Moon 随机生成 Spark 的条件；该分支由明确隔离输入和混合连续流完整验证。

## 复跑

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_default_arena.py \
  --out .local/md-yuyuhakusho/default-arena-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestDefaultArena' ./games/yuyuhakusho/engine
```

Wasm 沿用研究工具说明的 Node 执行器。原版复跑输出可与同名夹具／manifest 比较；独立测试直接读取已保留的原输出，不需要 ROM 或内核。
