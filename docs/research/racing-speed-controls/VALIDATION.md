# 极速、电脑驾驶与开局视区验收

道路真实极速由 310 世界单位／秒提高到 900（324 km/h），前向阻力由 0.55 调整为 0.2，使上限实际可达。前端预测、自动挡、镜头和引擎音效归一化同步更新，速度表仍使用原来的单位换算。路外限速和制动优先规则保留。

电脑的直道巡航目标由 250 提高到 700，前方路径观察上限由 90 提高到 200，同时随速度提前避让；仍按曲率和道路边界减速，通过普通操作接口完成对局。霓虹城区两车实测最高约 247 km/h，旧策略同一场景约 85 km/h。

踏板的 DOM、视觉和键盘聚焦顺序统一为「手刹、刹车、油门」。移除自动加速入口与首次触摸自动踩油门逻辑；按住才加速，松开惯性滑行，仅操作方向盘不加速。

所有游戏由平台在初始布局稳定后定位到 `.game-main`，顶部留 8px。用户操作取消待执行定位，快照不会反复滚动。泡泡堂移除重复定位；俄罗斯方块限制本人棋盘的视区尺寸，保持宽高比例，使 HUD、棋盘和按键在验收视区内可见。

## 本地结果

产物位于 `.local/racing-speed-controls/`，可重复执行的脚本已入库。

| 验收 | 产物 |
| --- | --- |
| 赛车 5 地图 × 2～6 人 × 1／3／5 圈，75 组自然完整对局及 HTTP／恢复 | `racing-full-first.txt` |
| 赛车物理、HTTP、电脑、恢复和输入竞态检查 | `race-final.txt` |
| 公共前端、赛车与泡泡堂 151 项检查 | `frontend-final.txt` |
| 俄罗斯方块 13 项前端检查 | `tetris-frontend.txt` |
| 六席位 320px 首屏、桌面／手机踏板顺序、原生多指油门／刹车／手刹、松手滑行、重连、观众与自然结算 | `racing-browser/report.json` |
| 正常按键驾驶超过旧极速，松油门确认服务端 throttle 为 0 | `road-speed-steering/report.json` |
| 俄罗斯方块、泡泡堂、坦克大战、街霸方块、疯狂坦克、五子棋在 1280px／320px 自动定位与手动滚动保持 | `position-final/report.json` |
| 编译、边界、静态检查及素材摘要 | `build-final.txt`、`boundaries.txt`、`vet.txt`、`assets.txt` |

自动定位验收检查真实坐标，并检查实时游戏按钮底部未超出 844px 高度；不以 DOM 存在代替可见性。极短视区保留棋盘最小宽度和正常页面滚动。测试使用 Chromium 触屏仿真，不代表实体手机性能测试。

```sh
. scripts/env.sh
npm --prefix web run build
python3 -B scripts/racing/check-remake.py --out .local/racing-speed-controls/new-racing --skip-maps --tracks metro --players 6 --extended --laps 3
python3 -B scripts/racing/check-remake.py --out .local/racing-speed-controls/new-speed --skip-maps --tracks metro --laps 3 --speed-check
python3 -B scripts/racing/check-remake.py --out .local/racing-speed-controls/new-position --position-only
go test ./games/racing/... -count=1 -timeout=25m
```

本轮仅改变赛车规则、公共开局定位、泡泡堂重复定位及俄罗斯方块尺寸；其他游戏不运行规则或完整对局矩阵。失败清单见 [PLAN.md](PLAN.md)。清理路径记录在本地产物目录的 `cleanup.json`，只清理本轮失败运行、临时浏览器资料和重复发布包。
