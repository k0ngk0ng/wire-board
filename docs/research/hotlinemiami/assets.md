# 《迈阿密热线》地图与素材恢复

本次素材以公开存档的 GOG Linux `2.0.0.4` 包为研究样本，最终玩法由独立多人规则实现。原版媒体、PNG、公开图集清单和音乐均保留在 Git 忽略的 `.local/hotlinemiami/` 内，不作为 Go 嵌入内容，也不进入 Git。服务端计算必需的地图碰撞数据单独维护在 `games/hotlinemiami/data/maps.json`。

## 样本身份与证据范围

安装包 SHA-256 为 `8a7fad25d855f4769e89f4bd0ed5a72410906f040856ab225128a6daeb61577b`。安装器仅按 ZIP 容器读取，没有执行安装流程。

| 成员 | SHA-256 |
| --- | --- |
| `Hotline` | `e20f21bfac04ed3fc2c1a2edfb3b2b729141efa7667d1dcaafc557676280e52b` |
| `HotlineMiami_GL.wad` | `a9499384a2a30a969f1db67e9f9e6d767a48a031272cdef987f349a8d7095e40` |

REA 包清点证据延续方案文档登记的 `ev_537166a5cdcc5f3a1926546fd8cea5e75d0cd0e58bc6799de18aeffcdd225ba8`，完整导出位于 `.local/hotlinemiami/research/rea-evidence.json`。本轮尝试用现有 Ghidra 对完整 ELF 进行 REA 反编译，默认分析在 330 秒截止前未完成；错误与提供方诊断保存在 `load-room-data.rea.json`。不能将该尝试描述为已完成函数反编译。

后续格式恢复使用普通 ELF 符号、`objdump` 静态指令和原版字节数据。以下结果属于这条独立恢复流程，不冒称新增 REA 证据，也不代表已经录制原版应用的运行回放。

## 容器、图集和精灵

WAD 索引包含 633 项，头部声明的资源起点为 31,180 字节。索引项为小端路径长度、UTF-8 路径、资源大小和相对资源区偏移。解析验证索引结尾、重复路径、目录逃逸和全部资源范围。

`.phyre` 中的纹理数据为 RGBA8。实例表、对象与数组数据、指针修复表及最终原始像素分别保存；PNG 文件名表与纹理坐标记录的顺序不同，不能直接按序配对。最初的验证捕获了这个错误：`sprPWalkShotgun` 错配为 17 × 17，原版精灵记录为 44 × 32。

恢复管线读取原版 ELF 中 `0x08948446` 的 `DecompressFixups(PArrayFixup)` 代码，在 Unicorn 的独立 x86 内存中执行该纯解压算法。样本摘要必须匹配；模拟过程有固定内存区域和指令上限，不运行应用入口、不转交系统调用、不加载原版动态库。返回状态、修复表数量和压缩流消费字节必须一致，再按修复记录恢复真实 PNG 名称与图集坐标。

对当前样本恢复了 410 张 RGBA8 图像和 1,499 个图集条目，包含加载图等非游戏精灵。OpenGL 像素行方向转为 PNG 方向时同步变换裁切坐标。`hotline_sprites.bin` 中的原始宽高与旋转原点保留到运行清单，每帧均有 `originX`、`originY`。最终只将本游戏需要的原始像素无损重排到小图集，不让浏览器加载原版全部媒体。

独立 Pillow 解码验证了扩充后运行清单的全部 37 张 PNG，并逐项核对实际尺寸和 SHA-256；不能仅以提取器自带 PNG 解码器验证自身输出。报告位于 `.local/hotlinemiami/verification/assets/png-decode-report.json`。

## 六张地图

`GMKDataManager::LoadRoomData` 静态指令确定了房间记录结构：468 字节固定头部、每项 32 字节的图块数组、每项 20 字节的对象实例数组，以及每项 40 字节的背景数组。完整解析 156 条房间记录，恰好消费 `hotline_rooms.bin` 的全部 3,504,824 字节。

| 混战地图 | 原版房间 | 原始尺寸 | 碰撞矩形 | 门 |
| --- | --- | --- | --- | --- |
| `no-talk` | 17，`rmApartment1Upstairs` | 640 × 480 | 131 | 7 |
| `full-house` | 31，`rmHouse2Downstairs` | 640 × 600 | 175 | 7 |
| `push-it` | 33，`rmHouse3Downstairs` | 640 × 608 | 152 | 10 |
| `overdose` | 20，`rmApartment2Upstairs` | 640 × 480 | 164 | 9 |
| `tension` | 49，`rmHouse5Upstairs` | 800 × 640 | 185 | 6 |
| `neighbors` | 35，`rmBuildingFloor3` | 640 × 480 | 99 | 2 |

关卡身份还对照了对象精灵路径中的 `02_No_Talk`、`06_Full_House`、`09_Push_It`、`03_Overdose`、`05_Tension` 和 `08_Neighbors`，没有根据内部 `rmHouse*` 名称直接猜测。原版的 `rmBossClub*` 属于另一组关卡，不作为 `Push It` 的替代。

背景由原版图块与环境精灵合成，保留原坐标和图层关系。动态门和可破坏玻璃不烘焙进静态背景，交由实际规则状态绘制。原版敌人、通关触发器、剧情爆炸效果和固定武器不会进入静态战场。`Tension` 中默认资源状态可见的 `sprRoomExplosion` 已按此原则排除，避免将未触发的剧情爆炸烘焙成永久地图图案。

碰撞矩形从原对象的碰撞精灵或默认精灵包围框扣除原点后恢复。墙体、玻璃、门和家具分类单独保存；这是多人规则使用的轴对齐碰撞投影，不声称等同于原版所有逐像素碰撞与动态家具行为。每项规则保留 `sourceInstance`、`sourceObject` 和 `sourceSprite` 来源，Go 引擎生成时只收集计算所需字段。

新增的 8 个候选出生点来自真实原版地板图块的最大可达区域，检查与墙体及门的间距；出生点和武器刷新点是多人改编，不能描述成原版规则。源码 JSON 使用原像素，浏览器规则投影及 Go 引擎使用像素乘 256 的定点整数。门开闭、弹丸、攻击与得分由多人引擎独立执行。

## 八种造型与十二类武器

Jacket 使用原版身体与鸡头面具叠加，保留 7 种持械行走姿势。Biker 使用原版专属行走，Richter 使用原版行走与鼠头面具。保镖和教父缺少面向全部公共武器的通用行走，因此新增了明确的素材组合映射：

- 保镖使用原版稳定上身姿势与专属腿部，组合为 8 帧、32 × 32 的通用行走。
- 教父使用原版上身与通用腿部，组合为 8 帧、32 × 32 的通用行走。
- 胡子使用原版商店上身与通用腿部，组合为 8 帧、32 × 32 的通用行走。

组合只裁切和叠加原版像素，没有把对话头像当作俯视人物，也没有补绘后宣称原版提取。保镖和教父行走各有 5 个实际不同的像素帧，胡子有 4 个；来源和操作登记在运行清单的 `adaptation` 字段。新增制片人使用 `sprBoss1Walk`，对应 `Decadence` 房间 21 的 Boss 对象 192；专属对话头像不完整，因此选择图使用原版俯视待机姿势放大。新增流浪汉使用原版 `sprHoboWalk` 与 `sprHoboFace`。角色的公共武器叠加与动作映射属于多人适配，原版 Boss 专属能力不随造型带入。

武器图标取自原版 `sprWeapons`，分别为手枪帧 7、Uzi 帧 9、M16 帧 0、霰弹枪帧 1、小刀帧 5、棒球棒帧 4、武士刀帧 8、马格南帧 10、MP5 帧 12、双管霰弹枪帧 6、消防斧帧 15、砍刀帧 18。小刀图标早期按外形误选的帧 3 已纠正；原版 `scrCurrentWeapon` 的实际分支显示帧 3 对应钢管，帧 5 才对应刀。纠错生成新的内容摘要，不覆盖已发布的旧对象。房间展示图对透明边界进行裁切和整数倍近邻放大，保留像素形状；不以文字或通用几何图形替代原版参考图。

六张地图、八种造型的头像与实际俯视图、十二种武器的总览位于 `.local/hotlinemiami/verification/assets/contact-sheet.png`。新增精灵使用独立扩充图集；除上述小刀纠错外，原有 20 张 PNG、全部原音效与音乐保持原摘要，便于资源地址复用。

## 音效和音乐

运行清单提供 17 个事件键，包含 12 类武器与 `pickup`、`throw`、`down`、`kill`、`door`。声音来自 WAD 的原 WAV，手枪对应 `sndSilencer.wav`，新增枪械分别对应 `sndMagnum.wav`、`sndMP5.wav`、`sndDoubleBarrel.wav`；新增近战复用原挥击音效。可减小体积时使用 FLAC；每项验证解码 PCM 与原 WAV PCM 字节完全一致，并通过 ffprobe 核验采样率与声道。清单保留源 PCM 摘要、还原 PCM 摘要、位深与采样帧数。

仅加入一首原版 `Hydrogen.ogg`，保持原始编码字节，不进行二次有损转码。大小为 8,855,631 字节，SHA-256 为 `158d123a6e56cefacf8b9ff9e68466c9dfc3b643f6faa61acb37c910e582303c`。音乐使用独立 `music` 元数据，通过平台音频接口播放；当前对局加载清单对这一个文件统计真实字节，不预加载完整原版曲库。

## 复跑与检查

工具依赖在 `scripts/hotlinemiami/requirements.txt` 固定版本，安装目标、下载缓存与所有产物均在游戏工作区内。需要已有原版样本及从 WAD 保存的元数据；普通构建从平台资源清单恢复最终素材，不运行原版提取。

```sh
mkdir -p .local/hotlinemiami/tools/python .local/hotlinemiami/tools/pip-cache .local/hotlinemiami/verification/assets
python3 -m pip install --target .local/hotlinemiami/tools/python --cache-dir .local/hotlinemiami/tools/pip-cache -r scripts/hotlinemiami/requirements.txt
python3 -B scripts/hotlinemiami/test_extract_assets.py
python3 -B scripts/hotlinemiami/extract_assets.py
python3 -B scripts/hotlinemiami/test_build_maps.py
python3 -B scripts/hotlinemiami/build_maps.py
python3 -B scripts/hotlinemiami/test_finalize_assets.py
python3 -B scripts/hotlinemiami/finalize_assets.py
go run ./games/hotlinemiami/cmd/maps --check
```

失败场景在实现前记录于 `asset-failure-scenarios.md`，提取器、地图恢复和最终运行资源分别先建立测试，再实现代码。14 项隔离验证覆盖损坏容器、路径逃逸、源摘要、Phyre 图集修复、PNG 无损像素、原版房间完整解析、真实碰撞、出生点、剧情效果排除、扩充运行清单、旧资源复用和音频无损还原。它们只证明资源恢复流程的有限契约，游戏可玩性、延迟、加载门槛和真实输入仍由正式房间 E2E 验证。

本地素材源以完整原版样本与必要中间数据保留供复跑；临时 Python 下载缓存、Ghidra 空工作目录和截图调试中间件在交付清理时删除。不得将整个 `.local/hotlinemiami/assets/` 按目录盲目上传，正式资源仅按最终清单的精确引用发布并按摘要去重。
