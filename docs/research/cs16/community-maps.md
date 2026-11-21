# 冰雪世界与血战回廊原图接入

本次新增固定版本的 `fy_iceworld` 和 `cs_bloodstrike`，通过原 BSP30 几何、碰撞 hull、实体和纹理恢复。新增转换命令为 `python3 -B scripts/cs16/build_assets.py --add-maps fy_iceworld cs_bloodstrike`；验收入口为 `python3 -B scripts/cs16/test_community_maps.py`。

## 来源和静态事实

原 BSP 来自 `hpoon/HLDS-CS1.6` 固定 ref `ce852844b4adf87dfcc6782cfb9021adf0ab35f4`，与此前保存的 `eallion/CS1.6` 树中同名文件具有相同 Git blob。所需 NAV5、外部 WAD 和雪天天空由这两个固定归档提供；每个文件的仓库、ref、源路径、字节数、SHA-256 和 Git blob 单独登记在 `games/cs16/assets/community-sources.json`。旧四图的来源清单保持不变。

| 原图 | 字节数 | SHA-256 | 实体事实 |
| --- | ---: | --- | --- |
| `fy_iceworld` | 124792 | `d36fe2c8cb774a08c0ab25af403204162596d5729a74d22dd68946d382a69481` | CT／T 各 12 个出生点，24 个 armoury，1 个中央购买区 |
| `cs_bloodstrike` | 152380 | `4d274b42c9fea481333dafd53aaac8edc83dbd051511610c448a1b39378eeeba` | CT／T 各 10 个出生点，无地面武器或显式购买区 |

两图都没有炸弹、人质、出生装备、剥除武器或地图购买参数实体，因此使用 `elimination` 模式。血战回廊的 `cs_` 前缀不能作为人质目标的依据。原世界消息分别为 `cs_iceworld by Fantasy - for Counter Strike` 和 `FTK Blood Strike by Tim`；第三方归档没有建立地图作者签名。

冰雪世界的 24 个 armoury 均为数量 1，覆盖 MP5、TMP、P90、MAC-10、AK-47、SG552、M4A1、AUG、AWP、M3、XM1014 和 M249，各两处。缺省 item 按原枚举为 0，即 MP5。`worldWeapons` 保留原实体编号、原 item、武器 ID、256 倍整数位置、65536 一圈的朝向和数量。原 Z 为 -192，武器按原 toss／落地语义由引擎计算最终高度。

中央购买区原包围盒为 `(224,-800,-256)` 至 `(544,-480,-64)` 个 GoldSrc 单位，未指定队伍。购买判定保留实际区域；血战回廊没有显式购买区，沿用原出生区域购买逻辑。

无目标地图的时限与拾枪语义对照固定的 ReGameDLL_CS 兼容实现。该参考默认在无爆破、无人质目标且未开启强制回合结束时，倒计时结束不直接判某队获胜，仍等待歼灭。平台隐藏这两图的无效目标时长设置，活跃歼灭阶段显示当前目标，冻结和回合结束阶段继续显示倒计时；两图均不自动复活。

## 导航

第三方 NAV 文件为 GoldSrc v5，头部 BSP 字节数分别匹配 124792 和 152380。解析按固定 ReGameDLL 参考源码的 place directory、area、连接、hiding spot、approach 和 encounter 布局完整消费文件，并校验版本、计数、边界和连接引用。

原面积数分别为 94 和 81。面积中心及相邻面积 portal 转为人物碰撞中心：站立加 36 个原单位，蹲下加 18 个原单位，再按实际 BSP hull 验证；导出 255 和 155 个节点。冰雪世界为单连通图。血战回廊主连通分量有 119 个节点，全部 20 个出生点最近节点都在主分量；源导航中的高处和边角小分量保留，电脑须从当前可达区域选取路线。

## 来源恢复与增量发布

显式本地恢复入口 `python3 -B scripts/cs16/fetch_sources.py` 新增读取独立社区来源组，按每条文件声明的固定仓库和 ref 复核 URL，验证摘要后原子写入 `.local/cs16/research/`。`--check` 完全离线。普通 CI 只使用已生成 Go 规则和已登记 public 资源，不因此自动下载 BSP、NAV 或 WAD。

增量转换只追加两图和必要的新贴图／天空资源。原四图公开规则、绘制数据、预览、天空以及原 `MapsJSON` 行摘要逐项保持，模型、音频和顶层原始来源描述也保持不变。重复执行不会再次插入条目。

新候选清单位于 `.local/cs16/verification/community-maps/source-resources.json`，精准变更为 `resource-delta.json`。登记候选包含 1027 条，原 994 条中仅主 manifest 被替换，新增 33 个逻辑文件；按既有内容地址去重，需新增 28 个不可变对象，共 1203298 字节。新逻辑路径中的未变天空和贴图继续复用已有内容地址。发布、缓存准备和全局资源索引由发布流程处理。

## 证据和验证

- `community-maps/source-facts.json`：原实体、出生点、地面武器、购买区及来源摘要。
- `community-maps/conversion.json`：四项先失败后通过的转换 E2E、六图身份、旧资源保留与重复性。
- `community-maps/public-materialization.json`：public 1027 文件逐项字节数与 SHA-256 核验。
- `community-maps/fetch-green.log`、`source-green.log`：固定多仓恢复、离线复用、路径与摘要验证。
- `.local/cs16/research/community-rea-evidence.json`：REA 对分析者组装的四成员原始证据 ZIP 的完整静态清点。成员身份完整且无完整性矛盾；该容器不是原作者发行包。

REA 检查 `ev_2f17e070ffe68bc95df376411f5b1258e73106aee28db5e574cd2efd9fc81893` 和成员清点 `ev_ca320d37be87bade87c77ba9ce348f2064e6580b8f8ca190beddae8425039337` 支持成员完整性。BSP／NAV 语义由可复跑解析器核对；两者都不是原版运行行为证据。真实房间、电脑对局、WASM 运动和 GPU 画面由对应 E2E 继续验收。
