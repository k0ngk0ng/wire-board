# 规则事实目录

本目录是原创 Go 规则引擎与原创 Canvas 界面使用的纯 JSON 数据，不包含原游戏程序、脚本运行器或媒体正文。参考来源为 PokéPath TD v1.7.3，数值、名称、规则描述、地图坐标、波次名单与素材标识保持来源事实；对局状态、行动合法性、战斗、效果和电脑策略由本项目独立实现。

`catalog.json` 包含 47 地图、4007 波、517 宝可梦形态、257 收集编号、347 敌人、254 能力、85 被动、197 物品、39 成就、11 天气、62 弹道和 41 首曲目。`manifest.json` 记录原始数据文件摘要、目录摘要、各表数量及客户端拆分文件摘要。

## 数据结构

| 表 | 关键字段 |
| --- | --- |
| `species` | 形态 key、稀疏收集编号 id、中文名称、颜色、威力／攻击间隔／射程／暴击的基础值与每级增量、技能 ID、地形、进化、攻击类型、弹道 ID、图集 |
| `enemies` | 稳定 ID、生命、护甲、像素／秒速度、漏怪伤害、再生、状态抗性、隐身、首领／精英、掉落、被动 ID、图集 |
| `routes` | 稳定数字 ID、背景／效果图、画幅、原始浮点路径、镜像路径、出兵偏移、24 px 格网、部署点、棋盘房棋子、解锁星数、天气、奖励、逐波名单 |
| `items` | 稳定 ID、价格、等级、可装备／可卸下标记、条件、限制、标签、超级进化事实、说明与图标 |
| `abilities`、`passives`、`weather` | 稳定 ID、多语言名称与规则说明，以及额外纯事实 |
| `projectiles` | 稳定 ID、图集与额外纯事实 |
| `achievements` | 原始数字 ID、多语言名称／条件说明与图标，不携带玩家完成状态 |
| `songs`、`playlist` | 以音源 key 索引的曲目名称、播放顺序 ID 和音频路径，以及按原曲序排列的 41 个 key |

角色数值字段采用 `powerBase`／`powerPerLevel`、`attackMsBase`／`attackMsPerLevel`、`rangeBase`／`rangePerLevel`、`criticalBase`／`criticalPerLevel`。敌人速度换算为像素／秒，同时保留 `speedPerFrame`。所有几何坐标保留浮点，不按画面缩放改写。

升级费用没有独立的原始数据表，因此另以 `upgrade-costs.json` 固化公开数值规则的离线算术结果；参考文件与摘要记录在该文件中。`upgradeCosts` 的 `low`、`mid`、`high`、`veryHigh` 数组各有 101 项，索引 0 保留为 0，索引 1～100 是当前等级的单级升级价格。产品只查此数字事实表，不嵌入、执行或翻译来源的升级函数。`manifest.json` 的 `factFiles` 保存这份独立事实文件的字节数与摘要；规则来源摘要同时绑定它和全部研究数据文件，价格变化也会更新存档规则版本。

波次 `sequence` 只包含敌人 ID 或 `null` 停顿位；`preview` 只包含敌人 ID。技能、被动、弹道和进化都使用稳定 key 引用，不重复嵌入整个定义。未扁平化的原始事实放在 `facts`，其中跨表引用使用 `表名/key` 字符串。`collection` 是 257 个基础形态的 key；编号并非连续上界，`missingNo` 的编号为 404。

地图的 `musicId` 保留原播放顺序，`musicKey` 明确关联音源。音乐顺序与文件编号不总是相同，例如地图 9 的曲序为 3，音源为 `route10.mp3`；不能用 `musicId + 1` 猜文件名。曲目元数据随核心目录下载，音频正文仍按当前曲目加载。原站切歌使用整份 `playlist` 循环，不以地图是否解锁裁掉曲目。

棋盘房（地图 35）的 `chessPlacements` 包含 16 个棋子的 `key`、`species`、`x` 和 `y`；坐标为原 24 px 格左上角，中心另加 12 px。原 780 格混合数组仍保留在 `facts.chessTiles`。棋子数据只随完整单图下载，不进入核心地图元数据。晃晃斑的专用路径也保留在对应地图的 `facts.spindaWaypoints`、`facts.spindaWaypointsMirror`、`facts.superSpindaWaypoints` 和 `facts.superSpindaWaypointsMirror`。

宝可梦与敌人图集均为 8 行方向：下、右下、右、右上、上、左上、左、左下。弹道图集为 1 行。图集字段保存原图宽高、横向帧数、行数和精确帧宽高；9 个原始图集存在小数帧尺寸，使用 `fractionalFrames: true` 显式记录，不静默取整。

## 读取与重建

服务端通过 `engine.CatalogData()` 读取生成的 Go 常量。客户端先取 `/pokepath/v1/catalog.json` 的核心表与地图元数据，再按当前地图读取 `/pokepath/v1/routes/<id>.json`，不提前加载其他地图路径或波次。`/pokepath/v1/resources.json` 仅提供媒体和拆分目录的路径、大小、摘要、MIME 及图片尺寸，供按需下载与真实字节进度统计。

资源索引的 `aliases` 保存逻辑音效路径到实际无损文件的映射。`files` 只登记实际发布对象的大小、摘要与 MIME，不能用源 WAV 大小统计 FLAC 的下载进度。音频处理与逐样本验收见[素材说明](../assets/README.md)。

```sh
node games/pokepath/build/extract-reference.mjs
node --test games/pokepath/tests/native-reference.test.mjs
```

重建器只使用 TypeScript 语法树读取 `.local/pokepath/research/source/src/js/game/data/` 的数据字面量、数组展开和表引用。它不使用 `eval`、动态 import、VM 或原游戏类，也不求值函数调用和控制流。歌曲只读取 `songs` 字面表，以其 `order` 事实归一索引；不执行来源中的映射或排序程序。缺少研究输入时明确失败，正常构建和运行只依赖已提交的规范化数据，不需要研究目录。

测试先行与导出结果位于 `.local/pokepath/verification/native-reference/`。测试检查全量计数、稳定引用、速度单位、路径精度、图集尺寸、客户端按图拆分和实际发布文件摘要。

## 规则修订身份

`engine.NativeRulesRevision` 由全部引擎产品 Go 文件经过 `gofmt` 归一后生成；排除 `_test.go` 和摘要文件自身，避免自引用。`engine.RulesVersion()` 同时包含程序修订摘要与事实来源摘要，不能用研究数据摘要代替程序版本。

`rules-revision.json` 同时保存在本目录和公共目录，仅携带版本、路径、大小及摘要。它绑定引擎输入、完整事实目录，以及核心目录和 47 个单图文件的客户端数据摘要；资源索引再引用这份清单，避免循环摘要。

```sh
node games/pokepath/build/rules-revision.mjs
node games/pokepath/build/rules-revision.mjs --check
node --test games/pokepath/tests/rules-revision.test.mjs
```

事实重建完成后会刷新修订清单。代码、数据或生成文件变化后，`--check` 拒绝旧产物。开发中的摘要只代表当次源码快照；完整路线验收和正式构建必须在规则落稳后重新生成并检查。
