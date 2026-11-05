# FC／MD 公共控制器接入验收

公共控制器已接入《魔强统一战》的独立配置。保留 FC 与 MD 六键预设，设置列表不提供 SFC。默认键盘仍为 W／S／A／D、F、空格、J／K／U／I，并增加 L／O；MD 中 U／I 为普通 X／Y，FC 中仍为 Turbo B／Turbo A。

`internal/gameapi.Module.Controls` 只声明已经接入游戏输入的操作。服务端以 `game:yuyuhakusho` 保存游戏配置，`md-6` 作为布局与一次性迁移来源。迁移后保存键相互独立，不会继续共用可变配置。控制器设置优先选择当前游戏，同时保留 FC／MD 预设。

## 输入与保存契约

键盘、替代键、触屏和实体手柄统一输出逻辑按钮。游戏通过 `PhysicalController` 与 `mergeLogicalButtons` 合并来源，不伪造键盘事件。设置、编辑、失焦、页面隐藏、禁用和设备中断释放对应输入；恢复后先确认中立状态。

| MD 按钮 | 默认键盘 | 浏览器 `standard` 位置编号 |
| --- | --- | --- |
| A／B／C | K／J／L | 2／0／1 |
| X／Y／Z | U／I／O | 4／3／5 |
| Mode／Start | F／空格 | 8／9 |
| 上／下／左／右 | W／S／A／D | 12／13／14／15，以及左摇杆 |

位置编号依据浏览器的 `standard` 报告，不能据此推断某个品牌的原始编号。非标准设备先录入映射，未配置时不输出操作。

实体配置以 `standard:<id>` 或 `raw:<id>` 区分浏览器报告的型号，连接序号只标识本次连接。配置按用户、游戏、型号保存，键盘与实体映射独立。旧版键盘客户端省略实体字段时保留已有配置；显式空 `gamepads` 表示重置。键盘清除保留逻辑键及空字符串。服务端合并旧字段后再次校验，避免保留的替代键与新主键冲突。

## 验收记录

验收产物位于 [controller-physical-port](../../../games/yuyuhakusho/verification/controller-physical-port/)。

| 验证 | 结果与产物 |
| --- | --- |
| FC 键盘、通用物理采样、MD 普通键 | 20 项通过，`js-final.log` |
| 实际 HTTP／SQLite 配置、迁移、用户／游戏隔离、旧客户端与非法输入 | 7 项通过，`http-native-corrected.jsonl` |
| 相同 HTTP 场景的竞态检查 | 7 项通过，`http-race.jsonl` |
| TypeScript 与 Go vet | 通过，`tsc.txt`、`vet.txt` |
| 实际服务、鉴权、SQLite 与设置组件的浏览器 E2E | 通过，`settings-browser-verified/result.json` |
| 桌面与 320px 设置界面 | `settings-browser-verified/desktop-physical.png`、`mobile-320-keyboard.png`、`mobile-320-physical.png` |

浏览器验收使用实际 `server.New`、《魔强统一战》模块与控制器 API，外层 React 页面为本地测试壳。覆盖当前游戏优先、键盘录入与冲突、替代键、清除与恢复、录入忽略预先按住的输入、标准／非标准型号、死区、单型号恢复、重开持久化、用户隔离、设置拦截、中立状态、断开与 API 拒绝回退。该验收不证明生产主入口或真实对局 WebSocket；主入口与整个平台对局由主验收负责。

实际 Board、原生 Go 模块和 Go/Wasm 的画面响应与松开记录由 [platform-web/controls-first/result.json](../../../games/yuyuhakusho/verification/platform-web/controls-first/result.json) 提供。该记录同样明确使用虚拟 Gamepad API，不是实体品牌兼容性实测。

## 测试先行记录与限制

失败清单为 [CONTROLLER_PHYSICAL_PORT_FAILURES.md](CONTROLLER_PHYSICAL_PORT_FAILURES.md)。`test-first.json`、真实缺模块失败、HTTP 行为失败与符号失败保留了最初的测试记录。设置界面的真实先行失败见 `settings-current-game-red/result.json`：预期选择 `game:yuyuhakusho`，实际仍为 `classic-2`。

新增 MD 采样测试初稿漏写一个对象右括号，首次执行发生语法失败。MD 分支已经实现后才机械修复该括号，未更改断言。因此不把这组 MD 行为测试称为已经完成语义先红；原始字节、SHA 与修复记录见 `md-gamepad-test-before-syntax-repair.mjs.txt`、`md-test-syntax-lineage.json`。通用缺模块失败与 HTTP 保存行为失败均为真实、有效的先行失败。

浏览器脚本的调试调整仅涉及展开 MD 绑定明细、等待测试壳异步刷新后的实际采样帧，以及使用现有错误状态的正确选择器；没有为这些脚本问题修改产品输入逻辑。原始脚本与调整说明保存在验收目录。未使用 USB／蓝牙实体硬件，本次不列任何品牌为已实测兼容。

## 重复执行

从 worktree 根目录运行；Go 缓存与临时目录都必须留在工作区。

```sh
export TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry"
export GOCACHE="$PWD/.local/go-cache"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
go test ./internal/server -run 'Test(PhysicalControllerPort|ControllerProfile|ControllerMD)' -count=1
go test -race ./internal/server -run 'Test(PhysicalControllerPort|ControllerProfile|ControllerMD)' -count=1
node --experimental-strip-types --test web/tests/controller-input.test.mjs web/tests/gamepad-input.test.mjs web/tests/md-gamepad-input.test.mjs
web/node_modules/.bin/tsc -p web/tsconfig.json --noEmit
go vet ./internal/gameapi ./internal/server
python3 scripts/yuyuhakusho/check_controller_settings.py --label settings-replay
```

浏览器脚本使用独立 loopback 端口与工作区内的测试数据库、临时 profile、构建目录，结束后自动关闭服务和浏览器并删除这些目录。仅保留可复核结果、先行失败证据、截图和源文件摘要，不修改 Main、不提交或上传。

验收入口 `controller_settings_host.go` 使用 `//go:build ignore`，避免 `scripts/yuyuhakusho` 研究目录被 `go list ./...` 识别为普通 Go 包并混入 C 观察工具。脚本按文件名显式 `go build` 仍可运行。该支持文件修复保留先行失败 `support-package-red.log`，验证见 `support-package-verification.json`；11 个控制器产品文件摘要均未改变。全包枚举已排除研究目录，剩余错误为仓库根目录尚未生成 `web/dist`，由主构建处理。
