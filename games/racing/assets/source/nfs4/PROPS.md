# 原始可破坏物件与兼容转换

本轮以 `b8e73e14` 的原始资源和转换器为基线，不运行 Wine，不新增宽泛的 EXE 分析。原 FRD／CAN 字节与字段关联可核验；字段名称、动作用途和时钟解释来自 speedtools 0.38.0 的社区格式解释，不能称为 Windows 实机行为测量。

## 已确认范围

| 赛道 | 确认的 normal／type 2 | 排除的 billboard 继承 type 2 | 单列的 special／mass 物件 |
| --- | ---: | ---: | ---: |
| France | 29 | 59 | 6 |
| Coastal | 83 | 229 | 4 |
| UK | 108 | 72 | 12 |
| Park | 45 | 68 | 1 |

白名单同时要求原 `ObjectHeader.type=4`、所指 `ObjectAttribute.collision_type=2`，且头部浮点位置与属性定点位置相差不超过 0.01 米。265 个物件均满足，昼夜的原 segment／chunk／localObject 身份、位置和几何相同。France 的全局物件序号和属性 identifier 会在夜景发生偏移，不能用它们作为跨昼夜身份。

France 的 29 个物件都使用 POLE 与路牌贴图；UK 的 108 个包含 81 片 MESH 网状贴图平面和 27 个带杆物件；Park 包含 STRP 条纹平面；Coastal 还有 MENU／1RF2 等材质的小物件。分类只说明原几何／贴图，不把所有物件统称为路牌。

原 `ObjectAttribute` 是 20 字节记录，最后一个字节为 collision type。现解析器枚举将 0／1／2／3 命名为 none／static／destructible／unknown；原普通物件实测为 1 或 2。逐物件的原 header、attribute 和 collision byte 偏移、原文件 SHA-256、误差、几何摘要及当前碰撞映射保存在 [props-audit](props-audit/)；它们支持直接回读原字节，不依赖截图判断。

`FrdData._get_object_collision_type` 同样把 billboard 的 attribute index 0 关联到路段第一个 road attribute。许多 billboard 位置与该属性相隔几十米，继承了别处物件的 1、2 或未解释值；这不足以证明它们可撞或可破坏。它们不进入本轮白名单，旧路线及其既有碰撞字节仍保留。special／type 6 带有独立质量和尺寸，但本轮没有将它们混入倒牌动作。

地面多边形的 `collision_flags & 0x80` 是墙标记，`0x40` 是物件碰撞标记，`0x20` 是有限高度标记；这与上述物件 collision type 字节是不同字段。固定墙、护栏和 normal／type 1 不因“看起来像小物件”而被开放破坏。

## 目录契约

客户端 `props/<track>.json` 使用米，包含 `version`、`track`、旧 `geometry`、旧规则文件 `ruleSha256`、`catalogHash`、origin、原文件摘要、动作元数据和 props。props 的 `index` 是 0 起连续序号，按日景 FRD segment／chunk／localObject 升序冻结；原身份为 `<track>:s<segment>:c<chunk>:o<localObject>`。`source.dayObject`／`nightObject` 仅作各场景序号映射。`routeBarrier` 指原 `route.barriers[]`，`ruleEdges` 指按原 `nfs-rules.py` 顺序展开的完整服务器碰撞边索引。

[props-rules](props-rules/) 是只含数值和身份元数据的分米副本，供后端复制为新版本数据。它与客户端共用 catalogHash，同时保留 geometry／ruleSha256。catalogHash 为客户端结构在加入该字段之前，以 `json.dumps(ensure_ascii=False,separators=(',',':'),allow_nan=False)` 编码的 UTF-8 字节做 SHA-256，不含末尾换行。数值副本引用该语义身份，不把自己的文件 SHA 冒充同一个值；其真实文件 SHA 应单独校验。

现有 route、geometryHash、collision、天空、车型和音频不因本轮变化而重写。新 GLB 只把白名单物件从 static_objects 拆成 `prop_<index>`，其下为初始 identity 的 `prop_<index>_motion` 和原 mesh；wrapper 保留原 pivot。GLB 顶层记录 propsCatalogHash，节点记录 propIndex／propId。不减少三角面，不缩小贴图，不把数值副本或审计文件列入客户端发布清单。

## 原 CAN 动作与边界

四条赛道的 `TR02.CAN` 都是 128 字节，SHA-256 为 `12362a6e470d86e168f3e3e502e91220c08ac600b6823caf54e17392f6c6d0d5`；`TR03.CAN` 都是 348 字节，SHA-256 为 `d025154c7c48fecf23b0ab7330737aea34c449df9534508f391861d4d50d7a06`。原头部直接读出 6／17 帧、delay=6。现社区解释将它们绑定低速／高速破坏，使用 64 Hz 时钟，得到末帧时刻 0.46875／1.5 秒。

输出 clip 分别为 `prop_<index>_destroy_low_speed`／`prop_<index>_destroy_high_speed`。CAN 平移是局部相对偏移，不能减去赛道 origin；只驱动 motion 子节点，不改变初始 wrapper。四元数沿用既有离线适配器的归一化、逆向／X 反射约定，逐帧与原读数校验。clip extras 明确 `trigger:'collision'`、`loop:false`、`clampWhenFinished:true`，不能随环境循环动画自动播放。运行端按权威碰撞状态选择动作、播放一次并停在末姿态；恢复时可按事件经过时间定位。

原版触发速度、碰撞方向如何重定向、倒下后的进一步刚体运动和重置时机仍未实机证实。玩法如使用独立重建阈值，必须与“原版测得”区分。

## 重复执行

先行失败清单位于 `scripts/racing/nfs-props.e2e.py`。转换前对旧资产执行 `--check-current`，因缺少物件目录形成预期失败；最终产物和完整结果保存在 `.local/racing-nfs-reference/props-e2e/`。从仓库根目录执行：

```sh
PYTHONWARNINGS=ignore::ResourceWarning .local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-props.e2e.py
PYTHONWARNINGS=ignore::ResourceWarning .local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-assets.e2e.py --extended
```

前者验证真实来源白名单、昼夜映射、完整原边索引、独立节点／原像素／三角面、相对 CAN 动作、原路线及其他资产字节不变；后者继续严格检查原环境动画、全部图片、车型、路线和重复输出。离线验证不替代正式页面撞击、同步、恢复和帧率验收。
