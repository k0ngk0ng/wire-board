# 泡泡堂原版参考核查

核查日期：2026-10-20。目标是盛趣运营、Nexon 开发的 Windows 客户端「泡泡堂」，不以同人网页或 Flash 炸弹人作为原版。此文只记录公开官网文档和截图证据；未运行客户端，不能将官网截图标记为本机实测。客户端提取和运行由主调查另行记录。

## 核查清单与结论

| 问题 | 状态 | 可确认内容 |
| --- | --- | --- |
| 目标身份 | 已确认 | 用户指定官网提供 V333 Windows 安装程序；下载页标明 2026-09-28 更新、747 MB。 |
| 基本操作 | 已确认 | 方向键移动；空格放水泡；Alt 装备针、飞镖等道具；Ctrl 使用已装备道具。 |
| 水柱命中后是否直接死亡 | 部分确认 | 官方针说明明确存在「禁闭在泡泡」状态、可以自救；官方氧气瓶说明可以延长困泡时间。不能以命中即死亡代替。 |
| 困泡持续时间 | 未确认 | 当前官方文档没有给出具体秒数；不能将 5 秒、10 秒等经验数字当作已核实参数。 |
| 队友救援／敌人击破 | 部分确认 | 官方刺猬模式说明刺破水泡可以拯救队友；二手资料描述「泡封后击破」。普通角色的接触距离、救援触发与击破判定仍待原版运行／客户端证据。 |
| 基础道具 | 已确认 | 容量、射程、速度分开增长；详见下表。 |
| 小区 10 视觉布局 | 部分确认 | 官方「对战地图」明确推荐小区 10，配图左上角可见 10；画面可数出 15 列、13 行。中央马路、树篱、房屋和双色砖块均可见。逐格碰撞和掉落属性未确认。 |
| 实际对战手感／精确时序 | 未确认 | 本调查没有完成客户端运行，不能以静态截图证明移动、碰撞、爆炸时间或困泡时间。 |
| 图片、音乐复用许可 | 未确认 | 获取到了官方公开参考图，但没有发现允许在本项目再发布的素材许可。初次调查仅放研究目录；后续经用户授权的 v2 接入见文末。 |

## 官方来源

所有下列链接在核查日实际读取。`reference-evidence/manifest.json` 记录保留文件的 URL、SHA-256 和用途。原始官方 JSON 可能包含 HTML，但未作为同源网页公开。

1. [官网下载页](https://bnb.web.sdo.com/web7/index.html)。用户指定的入口。与旧 `pop.sdo.com` 不同，实测可以读取。
2. [游戏操作指南](https://bnb.web.sdo.com/web7/news_detail.html?id=121317)。文章发布时间 2012-03-14。页面通过官方 `newscontent.asp` API 获取正文，原始响应保存为 `article-121317.json`。键位图保存为 `official-controls.jpg`。
3. [对战模式](https://bnb.web.sdo.com/web7/gameinfo.html?CategoryID=3018)。其中「对战模式」文章 ID 为 `121355`。定义基本模式为玩家对抗；列出刺猬模式「死后变成可爱的刺猬！刺猬具有刺破水泡的能力，可拯救队友」。不能把这个特殊模式直接扩展成全部普通角色的精确碰撞规则。
4. [对战地图](https://bnb.web.sdo.com/web7/gameinfo.html?CategoryID=3019)。其中「对战地图」文章 ID 为 `121375`。原文：「有房子和商店的村子，在这小村子里玩游戏吧！(18)张小区地图，(2张)超级小区地图！其中最受欢迎的是小区10地图哦！」这描述属于历史资料，不代表当前客户端全部地图数量。
5. [实用功能道具](https://bnb.web.sdo.com/web7/download_2.html?name=costume&CategoryID=2995)。原始列表保存为 `category-2995.json`。
6. [游戏辅助道具](https://bnb.web.sdo.com/web7/download_2.html?name=costume&CategoryID=3043)。原始列表保存为 `category-3043.json`。
7. [常见问题](https://bnb.web.sdo.com/web7/download_2.html?name=question)。原始列表保存为 `category-2941.json`。含踢鞋、手套和初始水泡数量说明。
8. [音乐下载入口](https://bnb.web.sdo.com/web7/download_2.html?name=music)。由官网主导航直接链接，主调查可用于音乐资源比对；本调查没有下载音乐。

官方 API 格式：

```text
https://news-my.web.sdo.com/UniInterface/news/newscontent.asp?Type=json&Article=121317&ApplicationCode=28
https://news-my.web.sdo.com/UniInterface/news/newslist.asp?Type=json&CategoryCode=3043&pageSize=100&pageIndex=0
```

API 地址和域名迁移关系来自官网 `web7/js/getList.js?v=0815`，不是猜测接口。图片旧链接 `http://pic.static.sdo.com` 迁到 `https://pic.web.sdo.com`，这是官网脚本声明的映射。

## 已确认的操作与道具

| 项目 | 官方描述／可确认规则 | 官方文章 ID |
| --- | --- | --- |
| 放水泡 | 空格键放一个能够爆炸的泡泡 | 121317 的键位图 |
| 泡泡 | 水泡数量增加 1，达到角色最大值后无效 | 122049 |
| 力量水 | 水柱长度增加 1 格，达到角色最大值后无效 | 122058 |
| 溜冰鞋 | 速度增加 1 个阶段，达到角色最大值后无效 | 122053 |
| 大力丸 | 水柱长度达到角色最大值 | 122215 |
| 冰刀鞋 | 速度达到角色最大值 | 122238 |
| 针 | 禁闭在泡泡时使用，可以逃出 | 122025 |
| 氧气瓶 | Ctrl 使用，使禁闭在泡泡里的时间变长 | 122237 |
| 盾牌 | 2 秒内抵挡水柱；对战一局可使用 2 个 | 122326 |
| 踢鞋 | 增加踢泡能力；前方被障碍或另一水泡挡住时不能踢开 | 122217、123132 |
| 手套 | 增加扔泡能力；扔自己的泡需连按两次空格，放置到投掷间隔要很短 | 122216、123131 |
| 角色容量差异 | 官方列出部分角色初始可放 2 个泡，说明不能声称所有原版角色初始属性相同 | 124037 |

现有公开说明没有确认增长对应的移动像素速度、炸弹引信秒数、角色最大射程／容量、投掷窗口毫秒数、困泡剩余时长和救援后保护时间。这些属于运行核查问题。

## 小区 10 的可见布局

[官方原图](reference-evidence/official-district-example.jpg)，来源：

```text
https://pic.web.sdo.com/bnb/bnb/web4/intro/map/district/map11.jpg
```

原图为 619 × 569 像素。注意 `map11.jpg` 是素材文件名，不能据此断定是「小区 11」。其画面左上角显示 `10`，且被官方「小区地图／小区10」段落引用。此关联比文件名可靠，但仍应以客户端地图选择页复核名称。

从图像可见 15 × 13 格，地图图像大致位于 `(8, 36)` 至 `(608, 556)`，每格约 40 像素。以下是**可见物件转录**，不是已验证碰撞数据。坐标从左上角 `(0, 0)` 起算。`H` 为房屋，`T` 为树／树篱，`O` 为橙色砖块，`R` 为红色砖块，`C` 为木箱，`.` 为空地／道路；玩家遮盖格按可见地面推定为空地。

```text
.O R O R T . . C T H R H . H
.H C H C T C . . T R O . . .
.O R O O T . C C T H C H C H
CH C H C T C . . T O R O O O
OO R O R T . . C T H C H C H
OH O H O T C C . T O R O R O
TT T T T . . . C . T T T T T
OO R O R T C . . T O H O H R
HC H C H T . C C T O O R O O
OO R O R T C . . T C H C H C
H. H C H T . . C T O R O R .
.. O O R T C C . T C H C H .
H. H R H T . . C T O O O . .
```

为避免字符间距产生误解，机器可读的 13 行、每行 15 个字符版本保存在 `reference-evidence/district-10-visual-grid.json`。该转录供实现前人工复核，不能作为已经通过的规则夹具。树和树篱是否可穿、水泡能否炸毁／推动双色砖块、木箱是否可推动、出生格是否随机，均需要客户端证据。原图仅有两个玩家，不证明所有支持人数下的出生点。

最直接的设计约束是：小区 10 不是均匀棋盘墙加随机木箱。它有清晰的 3 列中央纵向道路、左右建筑区和第 6 行横向通路，不能只更名或仅复用随机墙体布局。

## 二手交叉参考与未知

浏览器读取了[维基百科「泡泡堂」固定版本](https://zh.wikipedia.org/w/index.php?title=爆爆王&oldid=93873578)，保存为 `wikipedia-20261020.txt`。其玩法段落写道：利用水泡困住对手（泡封），再刺破困住玩家的水泡（击破）获胜。此页自带来源不足提示，因此仅作为术语和玩法方向的二手交叉参考，不能用它证明精确时序或版本行为。

原版录像尚未获取可核验片段。本次 Google 检索遇到反自动化页面，Bing 检索结果以官网和小游戏集合为主；没有把这些小游戏当作原版运行。Nexon `ca.nexon.com/Guide/Basic/1` 实际返回 Nexon Home 通用页，未获得韩国客户端指南。

后续运行核查至少需要：同一角色空地移动距离与转向；放泡后离开再进入；水柱首次命中、重复命中和自然困泡超时；队友触碰救援、敌方触碰击破、针自救；救援后的水柱再命中；地图选择页的名称与原图对齐。每项都应保留原版截图／录像和相应时间点，再决定参数。

## 用户授权后的第三方网页参考

用户随后允许寻找历史客户端、私服客户端和网页还原版。本节记录原作者网页 Demo；不把它称为原版客户端。

| 项目 | 地址 | 本次验证 |
| --- | --- | --- |
| 原作者 H5 Demo | https://github.com/visolleon/bnb | 已下载 `gh-pages`，提交 `7070051995a5feab0ca83eabc708cdecda1e0e74`；已本地运行、移动、放泡、困泡并死亡。 |
| 原作者公开试玩 | https://visolleon.github.io/bnb/ | 实际打开，画面标题「泡泡堂 HTML5 Demo」；默认图为小区 13。 |
| 联机派生版 | https://github.com/SineYuan/BnBonline | 读取 README，前端来自上项，提供 Go 和 Rust 服务端；本次未运行。 |
| 后续 AI 改版 | https://github.com/tpoisonooo/BnB | 读取 README，有较多相同图音资源；自行修改角色平衡、默认海盗 14，本次未运行，未把其规则当原版。 |

原作者压缩包为 3,729,653 字节，SHA-256：`536ca0f8f7f05faf88fddeddc37cda6e676d75013e126fd2a5e59e6109ed24b4`。下载地址：`https://codeload.github.com/visolleon/bnb/zip/refs/heads/gh-pages`。源码和资源保存于 `.local/bubbleparty-reference/remakes/bnb-gh-pages/`，完整资源哈希清单为 `.local/bubbleparty-reference/remakes/asset-inventory.json`。该仓库没有 LICENSE 文件；README 没有证明各素材的原始提取过程或再发布许可。后续按用户授权选取图集接入 v2；授权状态和来源仍在素材清单中保留。

REA CLI 分析使用仓库内 `TMPDIR`，完整结果保存于 `.local/bubbleparty-reference/remakes/rea-visolleon-evidence.json`。Evidence ID：`ev_9bd3d2e2ef37e2c94d94ee337e6258f62e70982ff13cfee021f26a5773bc24a6`。目录摘要：`fba6a6de08e06e78a0c559a933e7ff84dd1f459eea17930193037b837c52f763`。这是静态 JavaScript 分析，不是官方客户端反编译。

可复用的资源定位（每帧尺寸来自图像头与代码切片交叉核对）：

| 文件 | 图集结构 | Demo 中的使用位置 |
| --- | --- | --- |
| `Pic/Role1.png` | 288 × 256；6 列、4 行，每帧 48 × 64 | `bnbRole.js`，行顺序上、下、左、右，步行动画每 60 ms 换帧 |
| `Pic/Role2.png` | 336 × 268；6 列、4 行，每帧 56 × 67 | 同上 |
| `Pic/Popo.png` | 132 × 41；3 帧，每帧 44 × 41 | `bnbPaopao.js`，每 200 ms 换帧 |
| `Pic/BigPopo.png` | 648 × 72；9 帧，每帧 72 × 72 | `bnbRole.js` 困泡与破裂；各阶段实际使用帧见代码 |
| `Pic/Explosion.png` | 560 × 200；14 列、5 行，每格 40 × 40 | `bnbPaopao.js`；上、下、左、右与中心分行 |
| `Pic/Town*.png` | 草地、房屋、树、木箱、双色砖等各自文件 | `bnbMap.js` / `bnbBarrier.js` |
| `Snd/bg.wav` | PCM，30.000 秒 | Demo 开始后循环背景音乐 |
| `Snd/appear.wav` | PCM，0.152 秒 | 代码实际放泡调用此声音，不是按文件名猜测 `lay.wav` |
| `Snd/explode.wav` | PCM，0.502 秒 | 水泡爆炸调用 |
| `Snd/get.wav` | PCM，1.000 秒 | 道具拾取调用 |
| `Snd/die.wav` | PCM，1.573 秒 | 主角死亡调用 |
| `Snd/save.wav` | PCM，0.470 秒 | 文件与枚举存在，但本次未观察到救援播放，不据此声称救援已实现 |

运行产物位于 `.local/bubbleparty-reference/remakes/`：`capture-demo.sh` 为可重放脚本，`local-demo-input.webm` 为移动与放泡录像，`local-demo-observations.json` 为每 50 ms 采样的只读状态与音效调用日志，`local-demo-bubble.png`、`local-demo-after-blast.png`、`local-demo-trapped.png` 和 `local-demo-death.png` 为实际画面。音效调用不等于扬声器录音，本次未以调用日志声称人工听辨已完成。

操作为 `WASD` 移动、`J` 放泡，与官方空格键不同。浏览器 CLI 的 `keydown` 对字母键产生 `keyCode: 0`，无法驱动这个旧 Demo；复跑脚本因此通过页面 `KeyboardEvent` 发送明确的 `keyCode/which`，未直接改写角色坐标、规则状态或调用放泡函数。`J` 由浏览器原生按键接口发送。实际从 `(0,0)` 移至 `(0,1)` 再到 `(1,1)` ；该移动尝试仍随后困泡死亡，未证明已成功避爆。独立原地放泡观察到困泡状态，之后死亡。所有这些只证明此 Demo 的表现。

该 Demo 的水泡引信为 3 秒、困泡超时为 3 秒；代码硬编码初始速度 4、射程 2、容量 2。这些与官方角色差异说明并不等价。重构时应借鉴图像层次、图集和动画呈现，不能直接移植其不完整规则。

官方音乐补充：`CategoryCode=3031` 返回 7 首曲目，含怀旧小区 `RetroVillage.ogg`。所有原始 HTTP 及同路径 HTTPS 链接在本次访问均返回空响应 403，未下载到可验证音频；失败结果与原链接保存于 `.local/bubbleparty-original/resources/official-music-manifest.json`，没有绕过验证。第三方仓库的 `bg.wav` 已取得，但其与官方具体曲目的对应关系尚未验证。

## 重构接入记录

后续按用户授权，将原作者 Demo 的 21 张 PNG 逐字节复制到 `games/bubbleparty/assets/public/bubbleparty/v2/sprites/`。音频优先使用主调查从 2002 年官方客户端 `Fx.idx`／`Fx.idd` 实际提取的 13 个文件，未采用 Demo 音频。来源、原数据偏移、转换方式和 SHA-256 见 `games/bubbleparty/assets/manifest-v2.json`；授权状态未改写为已获许可。旧 v1 没有覆盖，本次没有上传 OSS。
