# 客户端共享呈现状态与 schema 2

客户端已支持精确资源字节、共享的无像素呈现状态、原样 checkpoint 和可丢弃的下一帧预览。原生与 Wasm 各 49 条完整冻结轨迹（各 203,428 帧）、选定完整对局竞态和本地浏览器验收均已通过。本阶段不包含网络延迟、正式平台页面或音频合成验收。

## 状态与资源

`runtimebridge.Session` 的公共 `snapshot` 只保存规则字段。schema 1 使用旧的 `TileSlots`／HUD 像素；schema 2 使用唯一的 `renderstate.State`，包含精灵槽与带类型的 HUD 引用。两种呈现状态互斥。

schema 2 的规则推进调用共享 `sessiontick.Advance`。`World.GraphicCache` 由共享推进更新；客户端临时从上一帧槽和本帧回调事件生成画面，并核对最终槽与共享呈现状态一致。临时 HUD 像素由 `MaterializeHUDRefs` 生成，不持久保存，不通过像素相等反查引用。

`DecodeNetworkAssets` 接收原样解压后的 JSON 字节，包括末尾换行。规则、精灵、HUD、共享呈现 metadata、选中场地及魔性之森 aux，逐项验证 descriptor 中的字节数与 SHA-256。metadata 中规则、精灵和 HUD 的身份还必须与已解码资源一致。

`resourceid.Decode` 验证 schema 1 descriptor 的完整 13 项注册表、固定逻辑路径、六个资源字段、大小与摘要格式；未知、重复、缺少、null 或尾随内容均被拒绝。descriptor 自身的 SHA 需要由调用方的可信服务端 View／清单核验；资源摘要校验不代表外部身份认证。本地研究页面以随源码维护的 manifest 作为 descriptor SHA 锚点。

## Go API

| API | 成功结果与约束 |
| --- | --- |
| `DecodeNetworkAssets(AssetBytes, arena)` | 返回绑定精确资源身份的 `Assets`；失败不返回半成品。 |
| `Assets.RenderIdentity()` | 返回共享呈现 metadata 与关联资源身份。 |
| `Assets.ResourceDescriptorSHA256()` | 返回 descriptor 原始字节的 SHA-256。 |
| `NewSession(options, assets)` | 网络资源创建 schema 2；旧资源入口继续创建 schema 1。 |
| `StepControlled([4]Buttons, automated)` | 以平台席位输入和显式电脑位掩码推进一帧；错误不提交会话。 |
| `Step([8]Buttons)` | 保留旧调用；schema 2 仅允许低四个输入通道，并从当前 Match 控制状态推导电脑掩码。 |
| `Save()`／`RestoreSession(raw, assets)` | schema 1 保持旧字段顺序及字节；schema 2 保存共享引用和资源身份。 |
| `Checkpoint(epoch)`／`RestoreCheckpoint(raw, assets)` | 恢复独立权威状态；epoch 为 24 位小写十六进制，网络 frame 为十进制字符串。 |
| `Preview()` | 深复制会话；非终局预测 F+1，已清场终局保持 F；原会话状态与帧号不变。 |
| `Digests()` | 返回共享规则摘要和呈现摘要，供外层与服务端 View 比较。 |

schema 2 的两人原版角色槽为 `[0,3]`，三人为 `[0,1,3]`，四人为 `[0,1,2,3]`；`HumanChannel` 按平台席位为 `0..N-1`。游戏角色槽与平台席位分开使用。

checkpoint 验证资源、metadata、descriptor、场地、选项、epoch、帧号类型和呈现引用结构。有效但被修改的规则状态，仍需外层与服务端 View 的双摘要比较后接受。

## JavaScript 边界

`Present` 位图含有真实的 `18446744073709551615`。checkpoint 与 Save 必须跨 JavaScript 作为原样 JSON 字符串或 UTF-8 字节传递，不可先 `JSON.parse` 再 `JSON.stringify`。本地浏览器已验证原样恢复的完整字节相等，并验证 Number 舍入后的 checkpoint 被原子拒绝。schema 2 的 `View.frame` 是十进制字符串；旧 `tick` 只保留兼容用途，不用作网络帧比较。

本地研究 Wasm 新入口：

```js
const response = JSON.parse(window.yuyuhakushoWasm(
  JSON.stringify({ Op: "load-v2", Options: { Arena: selectedArena } }),
  rules, sprites, arena, hud, auxiliary, renderMetadata, descriptor,
));
```

七项均为 `Uint8Array`；无 aux 时仍传递空数组。资源不进入请求 JSON。旧 `Op: "load"` 保留原有兼容路径。新页面在解压前核验 gzip 字节数和 SHA，在解压后核验 raw 字节数和 SHA，再传给 Go 复验。

| Op | 主要字段／附加参数 | 返回值 |
| --- | --- | --- |
| `prepare` | `Options` | 权威 `view`。 |
| `ready`／`inspect` | 无 | 权威 `view`。 |
| `step` | `Held`；附加完整 `320×224×4` RGBA 数组 | 写入像素，返回权威 `view`。 |
| `preview` | 附加完整 RGBA 数组 | 非终局写入预测 F+1 像素；`view` 是权威 F，`preview` 是预览帧。已清场终局的两者均为 F。 |
| `save` | 无 | 原样 `snapshot` 字符串。 |
| `restore` | 原样 `Snapshot` 字符串 | 替换会话并返回 `view`。 |
| `checkpoint` | `Epoch` | 原样 `checkpoint` 字符串。 |
| `restore-checkpoint` | 原样 `Checkpoint` 字符串 | 校验成功后替换会话并返回 `view`。 |

加载顺序为资源校验 → `prepare` → 可丢弃的 `preview` → 权威 `ready` → 实际推进。初始预览为预测第 1 帧，权威保持第 0 帧。checkpoint 仅保存权威状态，没有上一帧绘制事件；非终局恢复预览也遵守 F+1 语义，不声称重建 F 的精确画面，不改变音频或网络确认游标。

终局已通过实际像素核验。49 条冻结轨迹含 42 个自然终局与 7 个固定 pan 非终局端点。42 个终止 tick 均由 manager `0x8040` 的阶段 26 执行 `clear-world`，清为阶段 0，停止显示并完成淡出；该帧没有 draw 事件。原样恢复 checkpoint 后，42 个 `Preview` 的像素与对应实际终止帧逐字节一致，均保持帧号 F。此前仅由空事件推断「恢复后会丢失角色」不成立；无需新增绘制历史或近似终局状态。

## 验收与复现

先行失败范围见 `runtimebridge/SCHEMA2_FAILURES.md` 和 `runtimebridge/testdata/browser-failures.md`。首次缺少接口和首次浏览器仍为 schema 1 的真实失败日志均保留。一个原本合法的 Present 负例已修正为结构确实无效的 fixture，原测试副本、差异原因及 SHA 记录在 `schema2-invalid-present-lineage.json`，未放宽实现校验。

- 原生 49 条完整冻结轨迹，逐帧核验 world、sprite slots、材质化 HUD 与画面的累计摘要；含中途恢复与非零 pan 恢复。
- 原生竞态与 Wasm 边界覆盖旧 schema 1 的 10 份字节基线、八场地资源身份、错误原子性、HUD 引用、checkpoint 和 Preview。
- 浏览器覆盖真实 MaxUint64、精确大帧号、30 帧重放、损坏 gzip、错误／归一化资源、场地按需加载、可信键盘移动、隐藏释放和 320px 三指触摸。
- 另行采集全部 49 个端点的最后一帧事件、manager 阶段与 checkpoint 恢复预览；42 个自然终局的像素差异数为 0。每条重放还核对冻结的最终 world 与完整 pixel trace 摘要，保留真实终态样例的 checkpoint、前一帧、事件、RGBA 与 PNG。

浏览器复现命令：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_runtimebridge_schema2.py
```

终局证据复现命令：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/probe_client_terminal.py
```

终局结果与样例位于 `games/yuyuhakusho/verification/client-terminal-preview/`，`command.json` 记录执行前后的来源 SHA。临时 probe 仅导入现有独立 Go 规则与渲染包，未载入 ROM、模拟器或原版 CPU。

浏览器结果与截图位于 `games/yuyuhakusho/verification/client-bridge-schema2/`。原生／Wasm 命令和 JSONL 轨迹位于 `games/yuyuhakusho/runtimebridge/testdata/`。最终来源 SHA 与验收摘要见同目录的 `schema2-verification.json`。
