# 平台控制器设置

状态：个人设置的服务端保存、界面和键盘映射／连发基础组件已实现，并通过 HTTP 和浏览器验收。热血篮球的输入消费随游戏状态机继续接入；实体手柄尚未接入。

产品入口统一叫「控制器设置」。游戏按自身玩法分类；热血篮球属于「体育」，不作为模拟器页面或模拟器类别出现。

每个用户拥有自己的配置，并按控制器类型／型号保存。键盘默认映射与未来实体手柄的按钮／轴映射分开，不能把设备型号和键盘方案合并成一个全局可变对象。切换账号不能继承上一账号的私有配置；用户修改一种控制器的映射不能覆盖其他类型。

经典双键控制器的默认键盘映射：

| 逻辑操作 | 默认按键 |
| --- | --- |
| 上／下／左／右 | W／S／A／D |
| Select | F |
| Start | 空格 |
| B／A | J／K |
| Turbo B／Turbo A | U／I |

用户可以更改并恢复自己的默认设置。实体手柄接入时使用同一套逻辑按钮，按设备型号和控制器布局选择映射。游戏模块解释 Start／Select／A／B 在当前玩法中的意义；公共层只处理映射、按下／松开、连发及输入来源，不包含篮球动作判断。

验收必须覆盖用户隔离、保存与重新登录恢复、切换配置类型、冲突按键、恢复默认、WASD 和 J/K/U/I 的准确映射、连发节奏与松手清理、失焦／隐藏／断开手柄，以及输入框和设置弹窗不能触发游戏动作。服务器只接受当前用户自己的配置写入；配置不进入公开玩家资料。

## 当前实现

- 顶栏「控制器设置」沿用平台弹窗。目前分别维护「经典双键（FC）」和「经典四键（SFC）」预设，新增型号可以独立扩展。
- SQLite 按 `user_id + model` 保存配置。`GET /api/controllers` 只返回当前用户的配置；`POST /api/controllers/{model}` 校验按键和修改版本，拒绝冲突键位、无效型号及过期写入。
- 默认向右始终是 **D**。验收会临时改为方向键证明自定义和持久化，再恢复默认；最终展示截图为默认 WASD。
- 连发默认每秒 10 次，可选 2～15 次。普通 A／B 与对应连发同时按住时，普通按键不会被连发间隙取消。
- `KeyboardController` 实例隔离各自的按键状态，更换配置和清理时释放旧输入。游戏接入仍须验证其实际失焦、隐藏和模态框处理，不能以组件检查替代游戏操作验收。

验证命令：

```sh
source scripts/env.sh
go test ./internal/server -run '^TestController' -count=1
node --experimental-strip-types --test web/tests/controller-input.test.mjs
npm --prefix web run build
go build -o .local/controllers/wire-board .
python3 -B scripts/accept_controllers.py
```

HTTP 覆盖默认键位、两个账号／两个型号隔离、过期写入、无效配置、公开资料不泄露配置及 SQLite 重启恢复。浏览器使用原生按键修改、保存／刷新／恢复默认，并验证 320px 布局。结果和截图保存在 `.local/controllers/`。

## FC 手柄界面

游戏内手柄和控制器设置共用 `FCController`，使用项目自绘的红白机红金配色 SVG 底图。布局为左侧十字键、中间 Select／Start、右侧两列按钮：上排 Turbo B／Turbo A，下排 B／A。连发键位于手柄内，以 Turbo B／Turbo A 标注。热血篮球比赛画面与手柄分区：PC 与手机的手柄均在比赛画面下方居中显示，手柄不占用比赛画面内部空间。

游玩模式只显示方向和逻辑按键名；Select／Start、B／A、连发名称沿用底图标注，避免重复印字。配置模式在对应控件上显示当前用户的键盘映射，点击后重绑。键位明细为只读列表；经典四键（SFC）继续使用自身的设置布局。

共享组件不解释游戏位值，也不建立连发计时器。游戏通过 `sink.pressPointer(id, buttons)` 和 `sink.releasePointer(id, cancelled)` 接收逻辑按钮；连发由公共输入层采样。十字区域通过 pointer capture 支持连续方向和斜向切换；中间区域松开方向，多指按钮独立保持。失焦、页面隐藏、禁用、配置改变和组件卸载都会释放已持有的指针来源。

SVG 来源记录见 `web/src/controllers/assets/fc-controller.source.json`。比例参考来自 Evan-Amos 的公有领域 FC 手柄照片，发布素材为自绘矢量图，连发位置按本平台约定安排。

运行 `python3 -B scripts/accept_fc_controller.py` 可复核共享组件及真实 Settings 组件。脚本在独立浏览器会话中验证原生鼠标拖动、原生多点触摸／取消、模式区分、重绑与模型切换；接口使用专用设置夹具。真实账号／数据库的保存隔离仍由 `scripts/accept_controllers.py` 验收。结果和 PC／320px 截图保存到 `.local/fc-controller/browser/`；实际 Go 对局的输入接线另由篮球 live 验收覆盖。
