# NFS4 离线转换契约

原始输入为已校验的四代试玩包解出的游戏目录。格式解析使用独立安装的 speedtools 0.38.0，仅在离线资源生成时运行；网页和服务器不依赖该 Python 包。转换器不复制其解析实现。离线适配工具使用 GPL-3.0-or-later，与该工具依赖同许可；产物不包含 Python 解析器代码，产品运行代码不导入该工具。

实现前失败清单：原文件缺失、大小写路径不一致、坐标轴交换或左右手系错误、UV 上下翻转、丢失透明遮罩、车轮并入车身导致不能旋转、地面归零错误、车型 CARP 升级版本被覆盖、路线点重采样或首点重复、道路高程被压平、起点朝向反转、GLB 与规则路线使用不同平移、夜间文件被误认为另一条赛道、对象/墙面碰撞静默丢弃、遗漏对象材质、原贴图被缩小、同一输入输出不确定、失败时产生部分清单、发布资源混入服务器规则文件。

转换命令为 `nfs-convert.py --game-root <原版目录> --out <新产物目录>`，可使用 `--cars P911 FM50 LDIA MCF1 CCOR --tracks France Coastal UK Park`。产物为 `cars/<小写车型>.glb`、`tracks/<小写赛道>.glb`、`tracks/<小写赛道>-night.glb`、对应的 `route.json`、`collision.json`、天际图、`cars.json` 和 `manifest.json`。GLB 使用 glTF 2.0、米、Y 向上、车辆 +Z 车头；场景保留原贴图分辨率、RGBA 像素和原版顶点光照，材质区分透明遮罩与双面。PNG 与无损 WebP 以最终 GLB 体积比较，仅在完整 GLB 更小时启用 `EXT_texture_webp`，解码后的 RGBA 必须逐字节一致。每车四个 `*_whl` 节点保持独立 TRS，整车底部在 Y=0，`groundOffset` 记录原车身中心高度。

路线顶层为 `{version:1,id:'france',name,origin:[x,y,z],bounds:{min:[...],max:[...]},length,points:[...],barriers:[...]}`。原版为左手系，先反射 X 转为 glTF 右手系，再平移：`输出坐标=[-原X,原Y,原Z]-origin`；`sourceTransform=[-1,1,1]` 明确记录反射，`origin=[反射后场景最小X-30,0,原场景最小Z-30]`。包含全部场景和物件的 X/Z 坐标至少留 30 米外边距。点使用原始 973 点，不重采样，首点不重复，包含 `{x,y,z,leftWidth,rightWidth,nx,ny,nz,fx,fy,fz,surface,segment}`。附加字段可提供路肩、墙距、道路横向向量、原碰撞多边形和起点，不能改变这些字段的含义。

`cars.json` 保留两车的名称、尺寸、轮心、`groundOffset`、全部四套 CARP 参数与原 VIV 成员摘要。默认版本为 `base`，不自动选择升级版本。

验收先写于 `scripts/racing/nfs-assets.e2e.py`，使用真实原版文件，从命令行入口生成完整产物并独立读取 GLB 二进制、图片、规则数据和摘要；重复生成比较字节摘要。产物仍须在正式网页检查可见性、朝向、透明物件和高程，离线通过不代替页面验收。

## 原车驾驶舱与视角

驾驶舱扩展的实现前失败清单见 `scripts/racing/nfs-cockpit.e2e.py`：空网格或假仪表台、外观三角面丢失、重复地面偏移、通用眼位替代原眼位、车身／驾驶舱贴图混用、源像素变化、原 VIV 成员缺少摘要及导出不确定。该 E2E 先在旧转换器上失败，再扩展 `car_export`。

每车 GLB 的 `vehicle` 仍只平移一次 `groundOffset`，其下的 `exterior` 保持原高精度外观三角面数量，`cockpit` 独立包含原 `dash.fce` 全部内舱部件与 `dash00.tga`。顶层 extras 附 `cockpitCamera:[x,y,z]`、`cockpitSource:[{member,bytes,sha256}]`、`exteriorTriangles`、`cockpitTriangles`；眼位变换为 `[-原X,原Y+groundOffset,原Z]`。不得通过减少外观几何或放宽总面数断言来掩盖内舱缺失。

轻量视角契约位于 `classic-camera-modes.ts`：0 近追车、1 远追车、2 引擎盖、3 原车驾驶舱。B 只切换瞬时观察方向，松开恢复当前 C 视角。F3 控制最大绘距，上界同时受原环境雾裁切约束；F7 后视镜为真实的后向第二摄像机，400 米范围，以小渲染目标左右反射后合成到画布，不能使用静态图片。性能计数包含主场景、后视镜场景与合成 pass。原键位依据为试玩 `readme.txt` 的 Controller/Key Mappings；没有据此声称视角数量、眼位 FOV 或镜头响应已获 Windows 实机测量。

## 原车循环音频

实现前失败清单：遗漏车型、把后方声场及非发动机音频全部装入初始包、BNK 补齐采样误作有效音频、循环端点越界或包含末尾填充、声道/采样率/位深改变、有损转码、循环裁切与来源无对应、归一化或淡入淡出破坏 PCM 一致性、负载表和滑行表互换、512 点转速表截断、用猜测音调冒充原版测量、不同 VIV 波形被意外共用、元数据大小摘要与文件不符、失败留下半份资源清单。

离线入口 `nfs-audio.py --game-root <目录> --out <新目录>` 仅选择 BNK 中 `patchnum<64` 的前方发动机音色，保留负载与滑行共享的完整原表。当前五车型共有 27 个必要短循环，均为单声道 22050 Hz、16 位 PCM。按源 BNK 的循环起点与包含式终点切出循环区间，生成无损 FLAC；公开元数据记录原区间、完整源 PCM 摘要与裁切后 PCM 摘要，平台循环播放整个短文件。不要把裁切区间摘要称为完整原采样摘要。

每车产物为 `audio/<车型>/engine.json` 和所需 `engine-<patch>.flac`，清单为 `audio-manifest.json`。每音色包含原 rootKey、pitchBendRange、detune、音量与原 load/coast 各 512 点 pitch/volume 表；可附数学推导的连续音调倍率。RPM 到表下标及音调的公开重建映射须标注为 `community-reconstruction`，尚未取得原版实际音频输出测量，不能标为原版运行已验证。原左右/前后空间混音、随机微调和精确对数音量包络不在这批最小循环资产里。

音频 E2E 先写在 `scripts/racing/nfs-audio.e2e.py`：从原 VIV 重新读出 PCM 及循环区间，独立通过 FFmpeg 解码 FLAC，逐字节校验 PCM、格式、帧数、表数据、文件摘要、有限体积和重复输出。正式网页还须验证平台 `audio.loop` 的加载、切换、静音与播放，不能新增游戏私有播放器。

## 自制双音喇叭

未查明原版 H 与 BNK patch 对应关系，喇叭采用独立自制音频并标注 `origin:original-synthesis`，不能称为原版音频。生成前的格式、削波、双频、循环接缝、摘要及可重复性失败清单与 E2E 位于 `scripts/racing/nfs-horn.e2e.py`。输出为 `audio/horn/horn.flac` 与 `horn.json`：350／440 Hz、22050 Hz、单声道 16 位、4410 帧完整周期，FLAC 解码后的 PCM 必须与合成采样逐字节相等。元数据含合成公式、CC0-1.0 来源说明、格式和两种摘要。

两项资源加入必选 scene 清单，通过统一 `fetchClassicFiles` 校验大小与 SHA-256，播放前再次校验准备包。`useClassicHorn(pack,cars,localSeat,localHorn,audio,running)` 仅管理平台 `audio.loop` 的 `racing-horn-${seat}`；本车使用 Board 已鉴权且生命周期唤醒后的 `localHorn`，覆盖滞后的权威 horn／protected，远端使用已应用 horn 且检查 protected。停止、完赛、移除车辆、更换资源及卸载释放循环和已校验 data URL 引用；不创建私有播放器。
