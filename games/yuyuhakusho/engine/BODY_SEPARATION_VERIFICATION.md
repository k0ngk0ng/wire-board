# 身体分离：实现前验证范围

目标为原版 `0x1ee1e..0x1eee0` 的完整调用。该函数消费上一帧提交的 `FFA500` 记录；本阶段不覆盖命中、伤害、投技或完整人物更新。

实现前失败场景：误用无序或对称配对；只比较地面低 word；遗漏队伍过滤；将相同 owner 的记录排除；byte 宽高求和没有回绕；word 坐标差、`-32768` 取负和带溢出的有符号比较错误；丢弃记录 X 的低 word；相等 X 时未回写当前记录；把每次移动额外的一单位误作一像素；改变后续有序配对的输入；覆盖未写字段；缺失 owner 时部分修改状态；修改调用者 map；观察器改变原版 3,600 帧基线；用逐调用结果冒充完整连续对局。

先采集自然调用与明确声明私有输入的原 CPU 边界，比较全部 owner 对象和完整 256-byte 记录区，再写独立 Go 测试并记录缺实现失败，最后编写规则。原 CPU 边界覆盖四人、多记录、同 owner、同队、跨线、满缓冲、零尺寸、字节求和回绕、坐标／位移回绕、相等 X 以及一单位分离。正式引擎不读取 ROM 或模拟器。

## 已验证结果

- 2,068 次完整自然调用，其中 2,066 次非空，249 次改变人物对象；自然片段没有 X 完全相等导致的记录回写。所有 3,600 帧 RAM／视频仍与未观察的原版基线一致，调用期间没有范围外写入或未完成调用。
- 159 个原 CPU 隔离用例完整一致。包含 24 种四人迭代顺序、满 16 记录缓冲和自然片段没有出现的相等 X 记录回写。
- 全部人物 256 bytes、记录区 256 bytes、计数完整比较，无字段掩码。独立 Go 原生竞态与 Node/Wasm 均通过；输入 map 不变及缺失 owner／无效计数的原子失败通过。
- 红阶段实际因缺少类型和 `SeparateBodies` 编译失败，先于实现；摘要、来源与测试输出摘要见 [验证记录](testdata/body-separation-validation.json)。

复跑原版采样：`PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_body_separation.py --out .local/md-yuyuhakusho/body-separation`。从输出目录复制同名夹具与 manifest 到 `engine/testdata/` 后，执行 `go test -race ./games/yuyuhakusho/engine -run '^TestBodySeparation'`；Wasm 执行 `GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine -run '^TestBodySeparation'`。Go 缓存与临时目录使用研究工具说明中的 worktree 内路径。

这证明完整分离函数的独立语义；仍不包含完整碰撞管理器时序、前后帧人物更新、攻击／防御、伤害或完整对局。
