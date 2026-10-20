# 疯狂坦克（fortressblue）验收记录

日期：2026-10-20。实现来源：用户要求重做「疯狂坦克」，使用《Fortress 2 BLUE》原版素材、地图与玩法。
素材取自官方网页客户端公开资源包，经用户明确授权仅在本私有平台使用；原始媒体不进入 Git。
解码方法、来源哈希与许可见 `games/fortressblue/assets/README.md`、`sources.json` 和 `manifest.json`。

## 素材取得与来源复现

Steam 免费客户端需要账号许可和安装，匿名下载无法取得安装包；因此改从官方网页客户端
`https://game.fortress2blue.com/` 下载 Emscripten 资源包
（`tankclient_emsc.c906c.data.00`～`.02`、`.wasm`、`.js`，共约 57 MB），并拆出
`Download/` 与 `Asset/` 中的原版 `.i16` 精灵、地图层和音乐来源。REA provider 未配置，
没有可用的 Steam 安装包可导入，所以直接对官方 Cocos/Emscripten/LH5 资源做格式逆向：
Emscripten file packager 分块 → LH5 解压 → 逐行 span RLE → RGB565 像素，地图碰撞由原版
前景层 alpha 生成。该路线得到的原版资源与 Steam 版共用同一套 CCR 素材。

`python3 -B scripts/fortressblue/verify-original-source.py` 会重新运行与素材生成相同的解码器，
从本地原始 `.i16` 重新生成全部媒体并逐项比对：**233 项通过**，包括 5 个官方客户端源文件的
SHA-256、17 张地图的碰撞位图/出生点/前景/背景/叠加层/缩略图，以及 14 种坦克的
idle/fire/die/move、炮弹尺寸与字节哈希。结果保存在 `.local/fortressblue/asset-redecode.json`，
证明线上媒体确实来自下载的原版客户端，而不是新绘或合成占位。

## 范围

- 2／4／6／8 人，红蓝两队各半；最多 4v4。
- 17 张原版地图（前景层碰撞、背景层、缩略图、原版音乐）。
- 14 种原版坦克（官方大厅属性：HP、防御、速度、主炮/特殊弹攻击、最大角度、射程）。
- 位图可破坏地形、风力、角度/力度、朝向、特殊弹、坠落、突然死亡、电脑/托管、观战、恢复。
- PC 鼠标瞄准/滚轮力度/点击开火、方向键移动与瞄准、空格开火；手机滑杆和按钮。
- 与服务器一致的落点预览、2.4× 坦克绘制、1200 世界像素取景、屏幕边缘敌人指示和小地图。
- 地图选择按 stage 排序；等待房间地图/规则修改必须保存，未保存时禁止开始。
- 原版坦克、炮弹、28 帧爆炸、地图与音效通过平台 `BoardProps.audio` 播放；CDN 优先、
  同源 `/fortressblue/v2/` 回退。

## 自动化检查

```sh
go test -count=1 ./games/fortressblue/...
go test -race -count=1 -timeout=10m ./games/fortressblue/...
go vet ./games/fortressblue/...
npm --prefix web run build
npm --prefix web test -- fortressblue
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/fortressblue/fetch-assets.py
python3 -B scripts/fortressblue/verify-assets.py --dist --oss
python3 -B scripts/fortressblue/verify-original-source.py   # 需要本地原版客户端资源
python3 -B scripts/fortressblue/check-browser.py
```

结果：

- 服务端规则：原版目录 17 张地图/14 种坦克、4×4 碰撞格、炮弹扫掠碰撞与地形停止、
  爆炸伤害/弹坑/掩码变化、红蓝顺序、选角超时、回合推进/超时、毒/火/爆炸/雷特殊弹、
  同时全灭、时间到同生命和局、坏存档/非法弹坑/错误队伍拒绝、电脑合法动作、快照后弹坑保持。
- 模块 E2E：2／4／6／8 人全电脑对局完成；额外对 **17 张地图逐一进行 8 人全电脑对局**，
  全部自然结束（队灭或时间到判定），无死循环。
- HTTP/SQLite E2E：新游戏默认上架；8 人房 + 7 名电脑、选坦克、移动、开火、错误席位/
  epoch/turn/角度/力度拒绝、服务重启保持对局；2v2 随机地图；团队结果、历史记录。
- 前端：8 项疯狂坦克目录/瞄准/弹道预览/碰撞位图镜像/素材 URL 测试通过；全部 459 项前端测试通过；
  构建通过；24 款游戏边界检查通过。
- 素材：178 个原版 WebP/MP3 文件通过字节数、SHA-256、WebP/MP3 头、时长校验；本地、
  `web/dist` 和 `https://cdn.ichenj.com/bga/v20261002-1/fortressblue/v2/` 三处字节一致。
- 来源复现：官方客户端 5 个源文件 SHA-256 与 `sources.json` 一致；从原始 `.i16` 重新解码
  17 张地图与 14 种坦克的 233 项媒体/碰撞/出生点结果与 manifest、catalog、tanks.json
  逐字节一致（`asset-redecode.json`，0 失败）。
- 竞态：`go test -race` 通过（服务端 + 模块/HTTP E2E）。

## 真实浏览器验收

`python3 -B scripts/fortressblue/check-browser.py` 启动临时生产式服务，通过共享「创建房间」
弹窗选择「疯狂坦克」和 stage-4「钢铁工厂」，设置 8 席后创建房间；在等待房间修改地图，
验证未保存时不能开始、保存后服务器收到新地图；随后添加 7 名电脑、开始游戏、选择原版坦克，
并用键盘、鼠标瞄准、滚轮力度、空格和点击战场完成开火。本地回退运行：
`.local/fortressblue/browser/4cb91ae6/result.json`（55/55 通过）。CDN 运行：
`CHECK_ASSETS_BASE=https://cdn.ichenj.com/bga/v20261002-1 python3 -B scripts/fortressblue/check-browser.py`，
结果 `.local/fortressblue/browser/07133c45/result.json`（55/55 通过）。

覆盖：

- 共享「创建房间」两步弹窗真实创建房间；地图下拉按 stage 1～17 排序，选择「随机地图」时
  只显示「随机地图」和 17 张可选说明，不冒充某张固定地图；选择 stage-4 后预览、
  等待房间和开局 `data-map-id` 都是 `stage-4`。
- 等待房间修改地图为 stage-2 时显示「地图或规则尚未保存」，开始按钮禁用；保存后服务器
  options 变为 stage-2；改回 stage-4 并保存后开局实际使用 stage-4。
- 共享玩家栏 8 席；14 张原版坦克卡；原版立绘实际加载。
- 选坦克卡/标题/说明与战场 HUD/开火按钮对比度 ≥ 4.5:1；桌面操作区宽度不超出棋盘。
- 原版地图前景、背景、爆炸图集、坦克 idle 精灵由本地或 CDN 实际加载；
  Canvas 像素多样性检查证明不是空白或纯矢量占位。
- 服务器倒计时按 `serverNow + performance.now() - receivedAt` 计算，不归零。
- **进入对局后的第一发**用空格发射：服务器 shot id 增加、`animating=1`，飞行期间
  Canvas 的 `data-projectile` 为 `sprite`，证明首发射击动画与炮弹贴图都实际绘制；
  随后等待电脑回合结束，人类回合正常返回。
- 键盘 ↑ 改变角度、→ 使服务器位置增加；方向键在游戏回合内被正确接管。
- 鼠标移动到坦克右上方后角度约 59°、朝向为右，落点预览有 136 个采样点；
  移到左下方后朝向变左、角度约 9°；滚轮增加力度且页面不滚动；点击战场由服务器返回 shot id。
- 炮弹以 1.9× 原画绘制，并带玩家颜色尾迹和光晕；动画结束后 `data-craters > 0`，
  证明爆炸真实擦除了原版前景地形；刷新后 `animating=0`，不重播旧炮弹。
- 坦克按 2.4× 原画绘制，桌面取景宽度 1500 世界像素；stage-4 开局 8 辆坦克全部在画面内，
  屏幕边缘仍显示其他敌人方向，小地图标记全部存活坦克。
- 地图选择预览保持原版 25:16 画幅（实测 1.5627）；战场按 16:9 取景但不拉伸地图，
  只做等比缩放和裁切。
- 320×844：无横向溢出，Canvas 实际宽度 ≥280 px（16:10 高 178.75 px），8 席玩家栏存在；
  滚动到操作区后开火按钮完整位于视口内（高 48 px），坦克仍可辨认。
- 截图：`.local/fortressblue/browser/4cb91ae6/desktop-select.png`、`desktop-board.png`、
  `desktop-first-shot.png`、`desktop-after-shot.png`、`mobile-320.png`、`mobile-controls.png`；
  CDN 运行截图见 `07133c45/`。

## 可玩性热修复（2026-10-20）

针对「按空格不能开火、看不到炮弹、地图比例不对、按钮难看」的反馈，定位到三个实现缺陷：

1. **首发射击没有动画**：进入对局时 `seenShot` 为 `null`，客户端把服务器返回的第一发
   （shot id 1）当成“刷新前已经播放过”，因此第一次按空格只更新了棋盘，没有炮弹动画。
   现改为 `seenShot = 0`，只有刷新时已存在的 shot id 才跳过。
2. **炮弹永远看不到**：`now` 使用 `Date.now()`，动画 `start` 使用 `performance.now()`，
   `elapsed` 变成巨大的正数，绘制直接进入爆炸分支并瞬间跳过飞行。现统一使用
   `performance.now()`；炮弹按 1.9× 原画绘制，并增加玩家颜色尾迹和光晕。
3. **地图取景与预览比例**：桌面取景从 1200 世界像素放宽到 1500，stage-4 开局 8 辆坦克
   全部可见；地图选择预览从 16:9 裁切改为原版 25:16；坦克标签改为屏幕恒定尺寸，
   缩放后仍可读。

回归测试新增：首发射击必须出现 `animating=1` 且 Canvas `data-projectile=sprite`；
桌面取景宽度 ≥1500；地图预览宽高比 25:16；人类回合在电脑连射后返回。
本地与 CDN 浏览器检查均 **55/55** 通过。

## 已知边界

- 队伍模型固定为红/蓝两队且人数相等；原版的 A–H 多队自由组合不在本期范围。2/4/6/8 人的
  1v1、2v2、3v3、4v4 均由平台席位自动分队。
- 原版道具拾取、商店、账号成长、语音和观战聊天不重做；保留核心炮战与特殊弹。
- 共享玩家栏在手机上占据较高区域，战场与控制面板需向下滚动；未用游戏 CSS 隐藏或重排公共界面。
- 结果浮层使用平台共享组件；棋盘保留最终局面，游戏不插入自定义横条。
- 原版媒体不提交 Git；本记录中的 CDN 对象为不可变版本，修改素材必须创建新前缀。

## 发布与部署

- 发布提交：`dd0914c6884f22c280a26305540ebebbbfb23621`，tag `v1.22.0`。
- main 检查：Actions run [39356191736](https://github.com/k0ngk0ng/wire-board/actions/runs/39356191736)
  成功，覆盖模块计划、前端、共享与 `Test and Linux amd64 binary` 构建。
- tag 发布：Actions run [39357127144](https://github.com/k0ngk0ng/wire-board/actions/runs/39357127144)
  成功，复用同一提交已验证的 Linux amd64 产物；Release：
  <https://github.com/k0ngk0ng/wire-board/releases/tag/v1.22.0>。
- 部署镜像：`ghcr.io/k0ngk0ng/wire-board@sha256:d771c87af2cb4aabb297fb1dfe84ba2c9a4c28de7ff0e6e3329a36ba09bbb6d5`
  （本地镜像 ID `sha256:a041090f…`），容器 `WIRE_BOARD_VERSION=v1.22.0`，健康状态 `healthy`。
- 生产检查：`/healthz` 返回 `{"ok":true}`；同源回退素材
  `/fortressblue/v2/maps/stage-1-fg.webp` 与 `/fortressblue/v2/tanks/1-idle.webp` 均返回 200。
- 上下架设置保持原样：`fortressblue=1`、`battlecity=1`、`bubbleparty=1`、`dota=0`；其余游戏
  设置未改动。`cdn.ichenj.com` 素材基址与 OSS 配置沿用既有 `.env`。
- 更新前在线备份：`backups/data-20261020T020910Z-2750517.tar.gz`；解包数据库
  14,512,128 字节，`PRAGMA quick_check` 为 `ok`（13 个用户、4 个房间）。保留紧邻上一份
  `backups/data-20261019T144149Z.tar.gz`，同样通过 `quick_check`；删除更早的
  `data-20261019T142744Z-1220702.tar.gz`。
- 镜像清理：`update.sh clean` 删除历史 `v1.21.1`，保留当前 `v1.22.0`、回滚 `v1.21.2`
  及 `wire-tv` 等其他服务镜像。清理后服务健康；磁盘 40G 中已用 25G（67%），备份目录 3.5M。

### v1.22.1 可玩性修复发布

针对「不能键盘控制、角度不能鼠标控制、画面奇怪、选了地图不对、按钮难看」的反馈，
修复了瞄准/操作、取景比例、地图选择确认和操作面板样式；验收见上文 50/50 浏览器检查。

- 发布提交：`91942d31db3397b1aca14116e67c98a8c91a5ef1`，tag `v1.22.1`。
- main 检查：Actions run [39360579713](https://github.com/k0ngk0ng/wire-board/actions/runs/39360579713)
  成功（共享平台、fortressblue、前端构建与 Linux amd64 产物）。
- tag 发布：Actions run [39361452048](https://github.com/k0ngk0ng/wire-board/actions/runs/39361452048)
  成功，复用同一提交的已验证产物；Release：
  <https://github.com/k0ngk0ng/wire-board/releases/tag/v1.22.1>。
- 部署镜像：`ghcr.io/k0ngk0ng/wire-board@sha256:c1b3618296024af2f381669763c04530fd98fb6735feff7364914108e2fd6d6e`，
  容器 `WIRE_BOARD_VERSION=v1.22.1`，健康状态 `healthy`，`/healthz` 返回 `{"ok":true}`。
- 线上 bundle 已确认包含「鼠标瞄准」「随机地图」「地图或规则尚未保存」；stage-4 前景/背景
  回退素材均返回 200；最近 10 分钟日志无 error/panic。
- 上下架设置保持：`fortressblue=1`、`battlecity=1`、`bubbleparty=1`、`dota=0`。
- 更新前备份 `backups/data-20261020T025620Z-2855209.tar.gz`（数据库 14,544,896 字节，
  `quick_check` 为 `ok`，13 个用户、5 个房间）；保留上一份
  `data-20261020T020910Z-2750517.tar.gz`（同样 `quick_check` 通过），删除更早的
  `data-20261019T144149Z.tar.gz`。
- `update.sh clean` 删除历史 `v1.21.2`，保留当前 `v1.22.1`、回滚 `v1.22.0` 与
  `wire-tv`；清理后服务健康，磁盘 40G 中已用 25G（67%），备份目录 3.5M。
