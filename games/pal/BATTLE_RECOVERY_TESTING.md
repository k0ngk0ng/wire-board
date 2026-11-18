# 战斗断线与后台恢复验收

使用普通体力的三队员战斗夹具、真实认证房间和单独的浏览器构建。通过 TCP 代理暂停单向传输或断开连接；通过切换浏览器标签让游戏页真正隐藏。

## 实现前失败场景

1. 法术列表的按键已离开浏览器、尚未到达服务器。断线后重放该按键，或因为按键仍按住而继续移动选项，改变原选择或推进菜单。
2. 服务器已接受对灵儿施放的治疗指令，浏览器未收到确认。重连后退回未提交的菜单、丢失指令，或同一指令被再次接受并重复治疗、扣气。
3. 游戏标签隐藏时，旧方向输入仍持续或在回到前台后重新生效；手机新的原版画面触摸无法继续。
4. 断线或恢复触发全局加载、画布失去就绪状态、整页重载、永久错误或电脑接管；游戏局面、操作权限或画布尺寸改变。
5. 仅看到 WebSocket 握手就认定恢复成功。需要核对实际加载的前端包、原版列表像素和权威菜单；后续由玩家操作另外两名队员，核对治疗真实效果、耗气和最终 3 文战利金。

## 修复与结果

旧版把连接和可见性变化当成了画布重新初始化的条件。两次断线及一次前后台切换共触发 6 次画布就绪状态退回；原游戏局面仍保留。修复后，连接或可见性变化会清除瞬时输入、触控手势和本地预测，继续使用同一张画布及已加载的资源。WebSocket 收到连接后的权威快照之前暂停提交操作。声音从当前音乐位置恢复，已经过期或已播放的音效不补播。真正续档的新加载仍执行原来的就绪流程。

| 场景 | 实际结果 |
| --- | --- |
| 法术列表按住方向键，从浏览器发出后在上行代理截断 | 服务器未收到旧操作。断线期间按确认无效，重连后旧键不连发，列表选择不变，画布像素与权威快照一致。新的桌面按键可用。 |
| 按住方向键后切到另一个浏览器标签，再切回来 | 标签确实进入隐藏状态，旧按键未继续发送，回到前台后列表仍正常。320px 手机触摸成功选中法术与灵儿。 |
| 治疗指令已被服务器接受，但确认在下行代理截断 | 重连仍保留原法术、对象和目标，下一位队员可以继续选招。治疗前后真气从 100 降为 94，与原法术表的一次耗气一致；灵儿体力从 28 恢复至 103，最终获得 3 文。 |

修复版在同一页面完成上述流程，未重新加载画布、触发全局加载或启用电脑托管。手机全屏画布为 320×200，无虚拟按键。两次真实保存续玩分别保留法术列表和已提交的治疗指令。桌面与手机另各完成一轮已发布 CDN 音频回归：保存续播、前后台切换、全局静音及旧音效防重均通过，未新增上传。

原版法术来自对象 296、法术 33，成功脚本 43016 加 75 点体力。同一轮敌方正常攻击可能影响最终体力，测试以原法术表的准确耗气、实际加血及正常结算共同判定。此处只覆盖所列断线、后台和原版战斗夹具；随机丢包、密集法术帧率及正式发布仍需独立验收。

## 复现

为每次运行选择新的作用域。以下命令在仓库根目录执行：

```sh
. scripts/env.sh
scope=recovery-repeat
recovery_dir="$PWD/.local/pal/performance/battle-recovery-$scope"
mkdir -p "$recovery_dir"
(
  cd web
  npm run build -- --outDir "$recovery_dir/web-dist"
  node --input-type=module -e 'import {build} from "esbuild";await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:"esm",target:"es2022"});' ../games/pal/tests/field-render-probe.ts "$recovery_dir/web-dist/pal-renderer-probe.js"
)
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=battle PAL_SCREEN_BATTLE_CURRENT=1 PAL_RECOVERY_SCOPE="$scope" go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=8m
```

夹具写出 `url` 后，在另一个终端执行 `python3 -B games/pal/tests/battle-recovery-browser.py recovery-repeat`。脚本输出浏览器实际加载的前端包摘要、协议事件、画面及结果。保存回归用独立作用域和同一构建，改跑 `PAL_RECOVERY_SCOPE=<作用域> python3 -B games/pal/tests/screen-battle-browser.py`。音频回归使用 `PAL_AUDIO_BROWSER=1 PAL_AUDIO_CDN=1 PAL_AUDIO_BROWSER_SCOPE=<作用域>` 运行 `TestPALOriginalAudioBoardBrowserFixture`，再用相同作用域执行 `audio-board-browser.py`。

修复前后的原始结果与截图分别保存在 `.local/pal/performance/battle-recovery-oracle-red/` 和 `.local/pal/performance/battle-recovery-verified/`。统一索引与已校验归档为 `.local/pal/verification/battle-recovery-stage.json` 和同名压缩包。
