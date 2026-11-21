# CS 1.6 原始资源与转换

转换入口为 `python3 -B scripts/cs16/build_assets.py`，验证入口为 `python3 -B scripts/cs16/test_build_assets.py`。运行前显式恢复固定公开来源的样本，或通过平台资源恢复工具恢复已经登记的发布输入。转换器不会自行下载素材，CI 不自动回源。

## 来源与身份

原始地图、WAD、MDL 和音频来自固定提交的第三方公开归档。文件路径、原始字节数、SHA-256 和 Git blob 摘要保存在源清单。四张地图均为 BSP30；模型为 MDL10；外部贴图为 WAD3。该清点与转换不建立 Valve 发行方签名，也不证明原版运行行为。

导航来自固定版本的第三方 PODBot v7 waypoint。转换器读取原始位置、节点标记、8 条出边及连接标记；用对应原图站立／蹲下 hull 核对节点。两种 hull 均为实体内部的节点被剔除，出边随之重映射；只能蹲下通过的节点附加蹲下标记。边保留原 waypoint 的跳跃、梯子与蹲行语义，实际通行仍由同一权威碰撞规则决定。

## 地图

`data.MapsJSON` 只包含服务端计算必需的数据：Q20 平面法线、256 倍整数坐标、BSP 射线节点、站立／蹲下碰撞 hull、实体区域、动态 brush、出生点、目标和导航。碰撞 surface 保留原始面多边形与材质表映射，供穿透判定按命中点定位真实表面。

动态 brush 的三个 hull 依次为射线、站立和蹲下。`origin` 是原实体原点，面顶点和 hull 保持原模型局部坐标；`bounds` 为初始世界包围盒。滑动门保存实际移动方向、尺寸扣除 lip 后的行程、速度和等待帧；旋转门移动向量为零，另保存原角度和轴标记。

原 `cs_assault` 没有营救 trigger 或营救点，按原规则使用任意 CT 出生点距离严格小于 256 个原单位的球体营救范围。该图导出 `rescueLegacy=true`，轴对齐 `rescueZones` 仅保留作预览和寻路候选；权威营救使用出生点和严格球距判断。仅刷新规则的入口为 `python3 -B scripts/cs16/build_assets.py --rules-only`，它更新地图规则、Go 地图数据及资源清单，同时保留模型、贴图、音频和命中盒规则摘要。

公开绘制数据按原纹理合并三角面，保留重复 UV、原平面法线及光照 UV。每个三角面按原面法线统一为逆时针，退化面被去除。纹理从地图内嵌数据或声明的外部 WAD 恢复；透明贴图保留调色板索引 255 的透明度。UV 投影按源 float32 舍入，避免单精度边界使光照块误增一行。当前光照图保留第一个原始光照样式，动态灯光样式尚需单独对照验收。

Three.js 加载地图纹理和光照 PNG 时使用 `flipY=false`，与原始像素上传行序一致。所有位置保持原始 GoldSrc 单位及 X、Y 水平、Z 向上；权威规则使用 256 倍整数。预览为实际地图三角面的俯视栅格图，采用统一分色。Aztec 额外保留原植物、骨架等模型及 27 个实体的位置和朝向。

## 模型与音频

`models.py` 导出自含贴图的标准 GLB，保留骨骼、原动画序列及各 blend，使用确定性 gzip 压缩。原 MDL 的顺时针前面按 Valve 官方 viewer 的渲染约定统一反转为 glTF 逆时针前面；默认使用单面材质，并保留源透明遮罩与混合标志。模型资源的 `bytes`／`sha256` 对应压缩载荷，`decodedBytes`／`decodedSHA256` 对应原 GLB；浏览器先验证压缩载荷，再解压和验证 GLB。

人物播放原 `idle1` 等动画后，原点就是 GoldSrc 碰撞中心。模型直接放在权威人物中心，不额外减去半身高度。`data.ModelRulesJSON` 导出八个角色及人质的原命中盒，以及站立、蹲下和各持枪 extension 中央 blend 首帧定点骨骼变换，独立于客户端模型载荷。人质保持其原脚底原点。

声音遵循 GoldSrc 搜索优先级：同名资源由 `cstrike` 覆盖 `valve`。原 PCM WAV 转为 FLAC，逐条解码核对采样率、位深、声道、采样帧数及 PCM 字节摘要。清单保留原始声音键，例如 `weapons/ak47-1.wav`，对应发布文件为无损 FLAC。

## 输出与发布

客户端产物位于 `.local/cs16/assets/`，清单为 `manifest.json`。地图绘制 JSON、地图规则 JSON、模型采用显式 gzip 载荷，图片采用 PNG，音频采用 FLAC。清单所有间接纹理依赖均在顶层 `resources` 登记。

`.local/cs16/verification/source-resources.json` 是平台资源登记候选，所有记录均须通过 `scripts.resource_store.validate_record`。显式 gzip 载荷的对象名为 `bga/cs16/<压缩 SHA-256>.gz`，MIME 为 `application/gzip`，不设置 HTTP Content-Encoding；客户端依资源描述显式解压。普通图片和音频也使用内容摘要地址。

媒体载荷、公开地图数据、原始样本和中间模型不进入 Git 源历史。正式服务器仅包含独立规则代码和计算数据；客户端载荷经过平台构建清单发布到 CDN。登记和本地构建不代表已上传或发布。

## 验证产物

- `.local/cs16/verification/assets/original-maps.json`：四图来源摘要、平面、节点、导航、三角面和资源验证。
- `.local/cs16/verification/assets/manifest-summary.json`：四图、八角色、24 枪及附属装备、模型与声音覆盖。
- `.local/cs16/verification/assets/repeatability.json`：相同来源重复转换的清单与规则摘要。
- `.local/cs16/verification/assets/model-refresh.json`：最终模型单独刷新重复性及地图／Go 规则冻结摘要。
- `.local/cs16/verification/assets/rule-refresh.json`：原图 legacy 球体营救标记、规则单独刷新重复性及模型／媒体冻结摘要。
- `.local/cs16/verification/models-tests.log`、`models-summary.json`：MDL 格式、骨骼、动画、缓存与压缩核验。
- `.local/cs16/verification/models-three-loader.json`：Three.js 实际模型解析及 AnimationMixer 核验。完整 GPU 画面和真实房间仍由浏览器 E2E 验证。
- `.local/cs16/verification/model-pose-glb.json`：48 个源 MDL 动作／帧的逐顶点 Three.js 蒙皮数值对照。
- `.local/cs16/verification/models-winding-summary.json`：格式绕序修正、材质标志及模型重生一致性。
