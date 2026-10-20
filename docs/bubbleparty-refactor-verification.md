# 泡泡堂重构与五款封面验收

日期：2026-10-20。完成本地代码重构和验收，未创建版本 tag、未发布、未更新服务器。模块 ID 保持 `bubbleparty`，显示名称改为「泡泡堂」。

## 本次交付

- 修复地图 JSON 与 Canvas 类型不一致导致的隐形墙／箱，修复离泡穿透、连爆遗漏和持续水柱漏判。
- 新局使用连续服务端移动与原生键盘、鼠标、触屏输入；支持方向叠加后恢复、松键／手势取消停止，短放泡不会被在途请求吞掉。容量或冷却失败的放泡不再数秒后自动执行。
- 增加困泡、队友接触救援、敌方接触击破、困泡超时淘汰及救援保护。困泡与救援清空旧控制，防止获救后滑行。
- 新局不缩圈，到时双方仍有人存活则显式和局。旧存档保留旧运动、水柱、护盾、缩圈、人数判胜和时间戳。
- 小区10 按官网图重建住宅、道路、树篱、双色砖与木箱；其余三张为保留的自定义地图。2／4／6／8 人均有出生安全和逃生验证。
- 接入 21 张角色／水泡／水柱／地图图集与 13 个历史客户端音频文件，提供同源回退和无图矢量回退。音效沿用平台统一播放器与静音。
- 五款封面全部重画为独立手绘 SVG：俄罗斯方块、街霸方块、泡泡堂、疯狂坦克、坦克大战。没有修改其他四款游戏的玩法。

## 已完成的验证

| 范围 | 命令／方式 | 结果与证据 |
| --- | --- | --- |
| 泡泡堂规则、HTTP、存档与完整电脑对局 | `go test -count=1 -v ./games/bubbleparty/...` | 通过；4 张地图 × 2／4／6／8 人完整对局全部完成；`.local/bubbleparty-refactor-20261020/control-compat-full-tests.txt` |
| 竞态检查 | `go test -race -count=1 -v ./games/bubbleparty/...` | 通过；`control-compat-race-tests.txt`，完整地图矩阵在非 race 作业验证 |
| 输入、图集、音效和动画时钟 | `npm --prefix web test -- bubbleparty` | 14 项通过；`frontend-tests.txt` |
| 其他四款封面的前端范围 | `npm --prefix web test -- <tetris / spf2t / fortressblue / battlecity>` | 分别 12／10／10／6 项通过；`cover-*-frontend.txt`；未运行它们的规则／完整对局矩阵 |
| 素材 | `python3 -B scripts/bubbleparty/check-assets.py` | 34 个文件格式、尺寸及源／输出 SHA-256 通过；`assets.txt` |
| 编译与边界 | 前端 build、`go test -run '^$' .`、`check_game_boundaries.py` | 全部通过；`build.txt`、`application-compile.txt`、`boundaries.txt` |
| 真实浏览器 | `python3 -B scripts/bubbleparty/check-refactor.py` | 26 项通过、31 张截图；最终目录 `.local/bubbleparty/refactor-browser-r6dba1e/` |
| 五款封面 | 真实组件渲染、公共 CSS 夹具、桌面／320px／缩略图检查 | 通过；`.local/covers-redraw-20261020/REPORT.md` 和对应截图／源文件哈希 |

先记录失败场景并编写回归，再修改实现；最初红测、最终绿测和源文件摘要保留在 `.local/bubbleparty-refactor-20261020/`。独立只读审查发现的延迟放泡、快速救援滑行和旧局兼容问题已补回归并修复。

## 浏览器结果的实际含义

`result.json` 的 26 项检查使用正常 HTTP／SQLite 服务、真实时钟与浏览器原生输入，不向页面注入假对局状态。它验证实际坐标、水泡实体和碰撞、困泡与到期结算，而非仅验证输入序号增加。

- 桌面及 320px 下真实墙／箱像素不同于地面，关键资源与控件能实际看见。
- 320×844 八人开局由产品一次定位到战场；无需测试滚动，整张 Canvas、四向键和放泡按钮同时可见。公共玩家栏保留在原位置，向上滚动可以查看全部席位；普通快照不会强制滚动。
- 键盘叠加、鼠标按住／释放、触屏拖动换向／取消均改变真实位置，并在释放后停止。本次本机输入到首次可观察位移约 101～144 ms，是观测值，不是公网延迟保证。
- 困泡、结算和关闭结果时棋盘文档位置不变；刷新保留 epoch、坐标和困泡期限；结算关闭不因普通快照重开。
- 结束后死亡区域在正常动态模式下、刷新后和下一快照之间像素哈希一致，旧死亡动画不重播。
- 正常放泡按钮正文／小字对比度为 5.55／5.41；禁用控件正文约 6.11～6.22，实际 opacity 为 1。
- 真实阻断了 21 张图集请求并确认 `Network.loadingFailed`，然后检查矢量回退可见。没有用被缓存命中的请求冒充加载失败。
- 原版背景音乐及开始音频可播放；平台统一静音使声音通道暂停并静音，刷新保留静音偏好。

建议查看最终目录中的 `playing-1280.png`、`eight-player-initial-playable-320x844.png`、`trapped-320.png`、`refreshed-final-death-320.png` 和 `blocked-sprites-fallback-320.png`。封面对比见 `.local/covers-redraw-20261020/desktop.png`。

## 来源与精度限制

历史客户端使用 REA 解包并按索引提取音频；图片取自已本地运行、经 REA 分析的早期网页还原图集。两者没有混称为官方客户端图片或项目原创。详见[客户端取证边界](research/bubbleparty/client-evidence.md)、[官方参考核查](research/bubbleparty/reference-findings.md)和[素材说明](../games/bubbleparty/assets/README.md)。

本次没有进入原版官方联网对局，也未证明官方已经停服。困泡 5 秒、保护 1 秒等为项目参数；部分地形碰撞语义和出生安全调整已在契约注明。原版完整商城道具、坐骑、全部角色差异不作为已还原功能宣称。资源再发布许可状态仍如清单记载，本次未上传 OSS。

## 清理

已停止本次本地服务与浏览器，删除独占数据库、浏览器 profile、socket、Swift／Wine／REA 临时运行目录、失败运行及被最终结果替代的中间浏览器产物。保留最终验收结果、截图、录像、复现脚本、原始来源包、资源哈希和研究证据。共享 Go／npm 缓存、旧版验收证据、用户素材、现有游戏文件和发布对象均未删除。
