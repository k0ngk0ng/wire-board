# 手机按键修正结果

平台按钮统一声明 `-webkit-user-select: none`、`user-select: none` 与 `-webkit-touch-callout: none`。泡泡堂、坦克大战、赛车、俄罗斯方块、街霸方块和疯狂坦克的屏幕操作容器使用 `data-game-controls`，覆盖箭头、方向盘、按键说明和按钮空隙。保留各游戏原有触摸手势、指针捕获、键盘焦点及页面滚动设置；不对正文或聊天禁选。

检查发现各游戏防护不一致：泡泡堂／疯狂坦克操作区没有禁选，赛车／俄罗斯方块只有局部禁选，坦克大战／街霸方块较完整。其他游戏的普通按钮和台球的 button 力度滑条同样受公共样式保护；没有因此扩大到无关游戏的规则验收。

## 证据

- `.local/mobile-controls/baseline-all/report.json`：未改动的 v1.24.0 前端构建在六款游戏、320px／390px 下运行。缺少禁选的实际可见元素每场 12～30 个（包含公共按钮）；没有在 Chromium 仿真中观察到最终文字选区，不能把它称为已复现实体 iPhone 的文字操作菜单。
- `.local/mobile-controls/after/report.json`：新构建六款游戏、两种手机宽度全部通过。原生长按、滑动、触摸取消及双指操作，最终选区为空、可见按钮与说明均禁选；聊天可输入并通过原生 Shift+左方向键选择文字。含 12 张手机截图、6 张桌面截图及原始输入／确认轨迹。
- `.local/mobile-controls/release-controls.json`：每款游戏首次 320px 输入的松手／取消检查通过。后续 390px 泡泡堂样本会被此前自己放置的泡困住，服务器按规则停止控制，不能将“最后一个包不是 neutral”误判为卡住按键。
- `.local/mobile-controls/frontend.txt`：公共前端与六款受影响游戏 197 项通过。
- `.local/mobile-controls/bubble-race.txt`：泡泡堂服务端及公开模块／HTTP／对局／恢复相关竞态检查通过。盾牌纠正和接触边界见泡泡堂对应报告；坦克方向的原 ROM 像素对照见 `../battlecity-directions/RESULTS.md`。
- `.local/mobile-controls/build.txt`、`boundaries.txt`、`bubble-assets.txt`、`battlecity-assets.txt`：构建、游戏边界和素材校验。

WebKit 长按保护通过平台声明落实，本次没有实体 iPhone／Safari 自动化证据；实际触屏操作证据来自 Chromium CDP，不将桌面仿真称作实体手机运行。

## 复跑

```sh
. scripts/env.sh
npm --prefix web run build
python3 -B scripts/mobile-controls/check-browser.py --out .local/mobile-controls/new-run
```

`--out` 必须是尚不存在的仓库内目录。夹具使用完整生产注册表、HTTP、SQLite 和真实时钟；退出时清理自己的 profile、socket、数据库及临时目录。留存报告与截图，不使用正在进行的生产对局验收。
