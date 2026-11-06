# 密室逃脱

2～4 个角色实时合作逃生。支持真人、电脑伙伴、主动托管、桌面键鼠、触屏、按游戏改键、投票暂停、自动保存与重连。平台只有一个「密室逃脱」入口，在创建和等待房间选择密室。

## 可选密室

| 密室 | 主路线 | 另一条路线 |
| --- | --- | --- |
| 封锁站 | 寻找保险丝与检修记录，恢复备用发电机，分处两室接通联锁，乘升降机离开 | 校准相互影响的排水阀与信号装置，接通排水，开启检修通道 |
| 沉没档案馆 | 校准排水，把两份密封档案运至水闸，乘救生艇离开 | 解开相互带动的三重星环，接通观测台与水闸联锁，乘穹顶吊笼离开 |

每局由固定版本的种子生成器安排 9 个房间、连通通道与校准目标。房间入口没有消耗钥匙的循环依赖。照明、排水和升降最多同时接通两路，配电取舍、备用电池、有限医疗、运输速度、险情位置和路线计划共同影响行动。地图编号留空则重新生成；指定相同编号可练习同一布局。

探索、标准、险境分别提供 25／20／15 分钟。蒸汽和潮水事故有 12 秒预警；电力与伤势影响生存，倒地队员可就近救援。全队倒地或超时共同失败。角色携带档案时减速，一次只能携带一份；运至撤离装置后交付。所有人交付携带物并在撤离室确认后共同获胜。

电脑读取与真人相同的公开投影，先发现线索，再操作机关；不读取未知房间或内部答案。队伍可以指定路线，电脑会调整配电与任务。呼叫会邀请电脑在标记房间短暂集合，救援与档案运输优先。取消托管保留进度并清除旧控制，离线不会自动托管。

## 操作

- WASD／方向键移动，点击地面移动，点击物件走近调查。
- E／空格就近交互；M 打开设施地图；J 打开调查笔记；Q 呼叫队友。可在平台控制器设置中修改。
- 手机使用方向键、点击地面和物件；地图中可前往相邻房间并选择撤离计划。机关、供电、补给和救援均提供触屏按钮。
- 左侧旋钮会影响其下游读数。按 1～4 循环调节，最终读数与找到的记录对应。线索共享到笔记，避免靠记忆重复抄写。
- 联锁需要两处装置同时有健康队员在附近，单个角色来回点击不能代替合作。
- 使用平台全屏和结算浮层；全屏保留地图、笔记、机关、补给和触屏操作。

## 美术与声音

原创场景与四套角色采用统一的青灰、黄铜和暖光规格。封锁站以玻璃控制室、管道与机房为主体；档案馆以书架、月窗、木作与天文装置为主体。背景、设备、四向角色与行走帧由用户指定的 `gpt-image-2.5` 经 imagegen CLI／当前 API 生成，原图在 `assets/source/`，提示词在 `assets/prompts.jsonl` 和 `assets/walk-prompts.json`。

`assets/prepare.py` 统一画幅、裁切精灵并保留透明通道；首张行走图通过外围白色背景清理修正导出遮罩。`assets/manifest.json` 记录原图与实际加载文件的 SHA-256、尺寸、透明通道和帧边界。界面封面为独立 SVG 绘制，不使用商业游戏截图。

原创机械泛音、提示音和环境声由 `assets/audio.py` 合成，通过平台媒体接口播放。FLAC 产物与原 PCM 逐字节校验，记录在 `assets/audio-manifest.json`；遵循全局音量、静音与暂停。客户端素材统一经过公共 OSS／CDN 发布流程，源码原图不进入客户端公开目录。

## 验收与复现

先行失败清单与冻结指标见 [ACCEPTANCE.md](ACCEPTANCE.md)。

```sh
. scripts/env.sh
go test ./games/escape/tests -count=1 -timeout=10m -v
node --experimental-strip-types --test games/escape/tests/*.test.mjs
python3 -B scripts/ci/scope.py run --game escape --backend
```

规则验收包含 54 组完整电脑对局，覆盖两种密室、全部人数、全部难度和两种撤离路线；另验证 2,000 个种子的连通性与前置条件、可见性、配电容量、耦合机关、险情、倒地救援、超时、暂停、接管与恢复。HTTP 验收使用正式服务器时钟和 SQLite，自然完成真人主动托管与电脑混合对局，并验证结算恢复。结果保存在 `.local/escape/rules/`。

构建后启动真实浏览器验收服务器，在另一个终端运行验收脚本：

```sh
. scripts/env.sh
npm --prefix web run build
ESCAPE_BROWSER_SERVER=1 go test ./games/escape/tests \
  -run '^TestEscapeBrowserServer$' -count=1 -timeout=100m -v
```

```sh
python3 -B games/escape/tests/browser.py
python3 -B games/escape/tests/interaction.py
# 性能脚本需要 Pillow；缓存与环境都留在仓库中。
. scripts/env.sh
python3 -m venv .local/escape-tools
PIP_CACHE_DIR="$PWD/.local/escape/pip-cache" .local/escape-tools/bin/python -m pip install Pillow==13.0.0
.local/escape-tools/bin/python -B games/escape/tests/performance.py
touch .local/escape/browser/stop
```

浏览器验收使用 agent-browser 与其拥有的 Chromium 页面，覆盖两种密室、桌面与 320px 手机、原生键鼠／触摸、全屏、重连及真实快慢连接。性能脚本通过测试专用 TCP 代理给单端增加 400 ms RTT，并关闭单端真实连接检查恢复；故障入口仅编译在测试服务器中。保留 Canvas 首次绘制测量、实际截图、ACK、消息大小和快端推进结果，不将 ACK 耗时作为本地响应。

最终验收记录与截图见 [密室逃脱验收记录](../../docs/escape-verification.md)。
