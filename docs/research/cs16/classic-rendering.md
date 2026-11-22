# 经典呈现反馈对照与验证

本轮使用既有第三方归档中的 MDL、BSP 和已发布本地模型，不新增下载。归档身份与限制见 `assets.md`、`.local/cs16/research/FINDINGS.md`。没有运行或反编译选定原版客户端；下面明确区分原始素材静态观察、参考实现与本平台呈现选择。

## 来源与修正

| 范围 | 可核验来源 | 实现 |
| --- | --- | --- |
| 开镜 | 固定 ReGameDLL 参考提交 `1f16a7f25c14225931f8df1abc51f68d18ded839` 的 `wpn_awp.cpp:75`、`wpn_scout.cpp:70`、`wpn_sg550.cpp:71`、`wpn_g3sg1.cpp`；90／40／15／10 为原 4:3 水平视场角 | 使用 `2 atan(tan(hfov/2) × 3/4)` 换算 Three.js 垂直角度，宽屏扩展水平视野；不将实际角度误作档位索引。默认 90h 对应 73.739795v，40h 对应 30.536943v，10h 对应 7.508333v。 |
| 小刀 | 原 `v_knife.mdl`：`slash1/2` 各 2 帧 @30 FPS；`midslash1/2` 各 66 帧 @55 FPS；`stab` 47 帧 @35、`stab_miss` 61 帧 @40。参考 `wpn_knife.cpp:323/518/546` | 左挥选完整 midslash，右键预测使用 stab_miss，权威刀命中／墙面接触才切换 stab。 |
| 枪身后坐 | 原 Glock 片段名为 `shoot/shoot2/shoot3`，双持为 `shoot_left*/shoot_right*`，原匹配器没有命中 | 按实际片段选择并轮换；武器切换后再消费待播放动作，防止动作发给旧模型。额外枪身位移最多后退 0.6 HL、上移 0.15 HL，150 ms 内恢复，仅用于即时呈现；镜头使用既有权威 recoil，不重复加算瞄准。 |
| 死亡 | 原 terror 模型的 `head`、`death1`、`gutshot`、`left`、`right`、`death2`、`crouch_die`；参考 `player.cpp:2975` 的命中部位选择及 crouch 覆盖 | 按权威命中部位选择原片段，按死亡帧寻址并保留最终姿势；每席位仅保留当前生命的模型，不增加无限尸体。缺少旧死亡时刻时直接最终姿势，不补播。尸体可见性继续服从服务器投影。 |
| 死亡抛动 | 参考 `player.cpp:2487`：前后抛 100–200 HL/s，命中速度抛动 200–300 HL/s；死亡 Think 在地面每约 0.1 秒减速 20；原爆炸上抛可约 836 HL/s | 使用已授权公开的 DeathV，256 倍定点只解码一次；800 HL/s² 重力、约 200 HL/s² 水平摩擦。碰撞检查按 100 ms 采样并插值，最多 3 秒，水平／上抛安全上限为 1024／512 HL。地图碰撞约束横移、顶部和地面；顶部接触后继续受重力，不冻结在空中。它是有界呈现近似，不是新的权威刚体规则。 |
| 死亡镜头 | 本平台呈现选择 | 读取自己的权威 deathAt，先给约 2 秒倒地视角，再沿用允许的活队友观战；无可观战者保持最终低位。重连旧死亡不重新倒下。 |
| 手电 | 参考 `player.cpp:6671/6853` 的 EF_DIMLIGHT／impulse 100，地图原材质为不响应 Three.js 灯光的 MeshBasicMaterial | 在现有材质加入只作用于当前视线光锥的动态分量，沿用烘焙光照与贴图。开关使用已公开的 flashlight 状态，不改变伤害、可见敌人或网络位置。光锥与衰减是本平台呈现选择，没有声称逐像素复现原引擎动态光。 |
| 落地 | 参考兼容 `pm_shared.cpp:2907` 及 Valve HLSDK `pm_shared.c:2681`：350 HL/s 冲击门槛、速度 ×0.013 的 roll | 权威 land 事件触发有限 roll，450 ms 恢复；重复／旧事件由既有游标拦截。 |

用户最新反馈亮度可以接受。本轮没有改变 BSP 光照色域、贴图色域、全局曝光、环境光或 lightMapIntensity；手电关闭时原渲染分量保持不变。

## 验证范围

失败场景先登记于 `games/cs16/tests/classic-render-failures.md`。纯模块与材质测试先 RED，再实现；Node 检查入口为 `web-classic-view.test.mjs`、`web-flashlight.test.mjs`，另沿用 `web-shot-effects.test.mjs`。

复跑 `node games/cs16/tests/classic-render-browser.mjs` 可加载当前本地 `web/dist` 的 Bloodstrike、人物和武器，产生真实 WebGL 像素与截图。脚本自行创建 `.local/cs16/verification/classic-render/`，使用独立 agent-browser 会话，并记录源码 SHA-256。`CS16_RENDER_BASELINE=1 CS16_RENDER_REPORT=browser-baseline.json` 使用已发布 `1aaa511957ada846364a9bab949b8806e3a3ea43` 的原 renderer，保留实际错误视角的 RED。

定向报告 `browser-final.json` 覆盖开镜、刀、开关灯、Glock、尸体、旧保存、300 HL/s 抛动、836 HL/s 顶部碰撞后落地，以及落地与死亡镜头。报告保留原始浮点坐标；边界验收按 HL×256 或角度×1,000,000 量化成整数，不通过容差扩大边界。截图按报告名分目录，防止 RED 被后续结果覆盖。

该夹具验证呈现与资源接线，不代替正式房间规则、六图联机性能、实体手机或 Safari 验收。作者代码的本次实际 lint 范围为 renderer、view-effects、flashlight、夹具及两个定向测试；不代表全仓库检查。
