# 六角色／默认场地本地研究页

当前可以在浏览器选择幻海、桑原、幽助、户愚吕兄、酎或户愚吕弟，与原版恢复的电脑策略对战。支持 2／3／4 人混战和 4 人组队，提供电脑演示。人物、特效、默认场地及 HUD 使用独立索引素材；对局由独立 Go／Wasm 规则运行，不加载 ROM 或模拟器。

本页用于本地验收，尚未接入平台房间、网络同步、托管及音效。其他角色和场地没有开放。

## 启动

在 `worktrees/yuyuhakusho` 根目录运行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/serve_local_player.py --port 8799
```

打开 `http://127.0.0.1:8799/`，等待资源、Wasm 和画面初始化完成，再点击「开始对局」。所有构建、临时文件和浏览器测试 profile 都留在当前 worktree。工具复用已有 Go、Node 和 `web/node_modules`；缺少依赖时会停止，不写入用户目录安装工具。

默认操作与已确认的 MD 配置一致：W／S／A／D 方向，J=B、K=A、L=C、U=X、I=Y、O=Z，F=Mode、空格=Start。U／I 为普通按键。触屏手柄位于比赛画面下方；失焦、页面隐藏和触摸取消会释放输入。研究页使用已确认默认配置；按用户保存的配置将在正式平台接入时读取。

## 可重复验收

```sh
# 五种全新规则对局；原生和 Wasm 完整状态轨迹一致。
sh scripts/yuyuhakusho/verify_chu_sessions.sh both

# 两种完整逐帧呈现对局；核对状态、像素和中途恢复。
sh scripts/yuyuhakusho/verify_chu_runtime.sh both

# 实际浏览器、原生键盘和三指触屏；使用独立会话并在结束时关闭。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_local_player.py

# 本地页类型检查。
node web/node_modules/typescript/bin/tsc -p scripts/yuyuhakusho/tsconfig.json
```

双角色完整对局与呈现矩阵使用 `sh scripts/yuyuhakusho/verify_fighter_sessions.sh both`，浏览器验收使用 `PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_fighter_player.py`。机器结果见 [fighter10-e2e](../../../games/yuyuhakusho/verification/fighter10-e2e/native/render/ten-chu.json)，浏览器保留[桌面截图](../../../games/yuyuhakusho/verification/fighter10-local-player/desktop.png)、[320px 截图](../../../games/yuyuhakusho/verification/fighter10-local-player/mobile-320.png)和[原生输入结果](../../../games/yuyuhakusho/verification/fighter10-local-player/result.json)。完整绑定摘要见[双角色验收](../../../games/yuyuhakusho/verification/fighter10-session.json)。旧酎配置的既有报告继续保存在 `session-e2e` 和 `runtime-e2e`，用于逐字节回归。

原视频对照分别证明规则调用和选定呈现帧；全新对局没有逐 tick 注入原期望状态。原生／Wasm 完整轨迹一致证明两个独立构建的确定性，不单独替代原版行为对照。

幻海父子对象与三角色矩阵使用 `sh scripts/yuyuhakusho/verify_genkai_sessions.sh both`，真实浏览器使用 `PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_genkai_player.py`。结果与[桌面](../../../games/yuyuhakusho/verification/genkai-local-player/desktop.png)、[320px](../../../games/yuyuhakusho/verification/genkai-local-player/mobile-320.png)截图见[三角色验收](../../../games/yuyuhakusho/verification/genkai-session.json)。规则包 schema2 增加 11 条公共 projectile 动画和 8 组释放短序列；旧角色回归仅规则数据摘要改变，世界及事件轨迹未变。

幽助与四角色矩阵使用 `sh scripts/yuyuhakusho/verify_yusuke_sessions.sh both`，真实浏览器使用 `PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_yusuke_player.py`。幽助阶段的[桌面](../../../games/yuyuhakusho/verification/yusuke-local-player/desktop.png)、[320px](../../../games/yuyuhakusho/verification/yusuke-local-player/mobile-320.png)和[完整结果](../../../games/yuyuhakusho/verification/yusuke-session.json)独立保留。

桑原与五角色矩阵使用 `sh scripts/yuyuhakusho/verify_kuwabara_sessions.sh both`；它覆盖 13 组全新规则对局和 7 组完整呈现。浏览器使用 `PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_kuwabara_player.py`，检查实际来源 2、前向攻击 `03/83`、原生键盘、320px 三指触屏、取消与后台释放，保留[桌面](../../../games/yuyuhakusho/verification/kuwabara-local-player/desktop.png)、[手机](../../../games/yuyuhakusho/verification/kuwabara-local-player/mobile-320.png)和[原生输入结果](../../../games/yuyuhakusho/verification/kuwabara-local-player/result.json)。

来源 8「户愚吕兄」与六角色矩阵使用 `sh scripts/yuyuhakusho/verify_elder_sessions.sh both`，覆盖 15 组全新规则对局和 9 组完整呈现；原生浏览器使用 `PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_elder_player.py`，当前[桌面](../../../games/yuyuhakusho/verification/elder-local-player/desktop.png)、[320px](../../../games/yuyuhakusho/verification/elder-local-player/mobile-320.png)及[完整绑定](../../../games/yuyuhakusho/verification/elder-session.json)独立保存。来源 8／10 的姓名现已订正为兄／弟，详情见[原字形与历史说明](CHARACTER_IDS.md)；旧阶段名称不作为当前姓名依据，数字 ID 与原规则证据未变。
