# 人物初始化：实现前验证范围

目标为 `0x126c8` 的公共初始化及 11 个角色分支，直至实际返回。独立规则接收语义状态与只读角色规格，不执行 ROM 地址，也不把初始化、房间出生、菜单和加载就绪混为一项。

可能失败的场景：遗漏属性 OR 或将其替换为赋值；清空图形缓存、偏移或时钟时覆盖相邻字段；把 HP、能量及显示 word 的写入当作完整 long 清零；错误恢复队伍、基础硬直和攻击随机系数；仅凭动画目录指针就声称完整默认人物；调用通用重置导致误清 `+60/+64/+6d/+77`；角色索引、相对分派基准、五个数据源或分支返回错误；把未知运动参数 `+10/+14` 猜为业务字段；夹具仅验证干净初态而漏掉残留字节；修改只读规格库；无效角色时部分写状态；原 CPU 观察改变 3,600 帧 RAM／画面基线。

先保存完整自然原调用与全角色 dirty-input 隔离结果，再写测试、保留缺实现失败，最后编写规则。逐次比较全部 256 bytes 人物对象，并核对除原调用栈外没有其他 RAM 写入。规格中只命名已确认消费的行走、起跳、重力、横速、蓄力速度与能量消减参数；未确认字节保留边界与来源摘要，不提前起业务名称。

## 规则接口与规格

`InitializeFighter(FighterInitializationState, attributeMask, FighterSpecLibrary)` 完整恢复公共部分与相对表的 11 个来源分支。来源 ID 0～10 与五个数据源对应，数据源只作不可变语义目录的身份，不在独立规则中执行或解引用。角色中文名称未从图像猜测。

公共部分对属性执行 OR，清图形偏移、参数、四缓存和上传跳过字节；时钟只写 index／duration／ticks／flags。生命 word 设为 223，显示生命 word 清零；能量只替换 long 的高 word 为 223，保留低 word，显示能量 word 单独清零；队伍恢复基础队伍。角色分支绑定五个来源，设置 action／pose 为 0、阶段 3，以及当前／基础硬直和攻击随机系数。

初始化不清 `Clock.Completed(+60)`、攻击属性 `+64`、动作标志 `+6d`、保留按下 `+77`、请求／命令、坐标、速度、重力、朝向、层级、深度及其他未写状态。不能用通用 `ResetActionState` 代替此过程，也不能把调用它视为完整出生或重置对局。

[fighter-specs.json](../data/fighter-specs.json) 保留 11 份规则规格及各自 30-byte 参数来源，`FighterMovementSpec` 只命名偏移 `0/4/8/c/18/1c`。偏移 `10..17` 的八个字节保留为未分配用途的来源区间，不把它们默认作为速度／时间／能量规则。

## 已验证结果

- 固定输入出现 2 次完整自然初始化，均为来源 ID 9；观察全部 3,600 帧完整 RAM／画面与未观察基线一致。
- 4,224 个原 CPU dirty-input 完整调用，11 个来源各 384 个。包含 16 种覆盖全部人物对象的脏字节模式、六种全局属性掩码、四种旧属性，另外显式变化 Completed、动作标志、基础队伍与五种能量低 word。每次比较完整 256 bytes，隔离调用没有原栈之外的范围外 RAM 写入。
- 独立测试检查只读库不变、缺失／错误来源的原子失败、初始化保留未写字段，以及六个已消费运动参数的独立 big-endian 原字节解码。真实缺类型／函数失败先于 Go 实现。

原生竞态和 Node/Wasm 聚焦验证结果及来源摘要保存在[验证记录](testdata/fighter-initialization-validation.json)。聚焦构建采用明确的生产／测试源码范围，避开并发子任务正在建立的缺实现红阶段；父阶段仍需运行整包集成门禁，不把聚焦结果称为完整人物或世界验收。

原版复跑：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter_initialization.py \
  --out .local/md-yuyuhakusho/fighter-initialization-replay
```

全部新组件齐备后，使用研究工具说明中的 worktree 内缓存，运行 `go test -race -count=1 -run '^TestFighter(Initialization|Specs)' ./games/yuyuhakusho/engine`；Wasm 使用同一测试筛选与既有执行器。当前仅证明初始化和已确认规格；出生调度、角色选择、房间就绪与完整游戏另行验收。
