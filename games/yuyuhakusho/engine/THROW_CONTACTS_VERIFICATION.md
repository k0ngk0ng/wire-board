# 抓取接触：实现前验证范围

本阶段恢复 `0x1eee2..0x1f08e` 的完整抓取形状消费和 `0x1f7b2..0x1f7d4` 的层级设置。先记录失败场景，再采集完整原 CPU 输入／输出、编写测试并保留缺实现失败，最后实现独立 Go 规则。抓取形状生产、后续投掷动作与伤害不属于本阶段。

可能失败的场景：将 A600 的 32 bytes 记录误读为普通 16 bytes 形状；X／Y word 或 byte 宽高偏移错误；漏掉目标延迟、同队、地面高 word、目标阶段 bit5 或双方帧标志 bit3 检查；把 word 差及取负、byte 尺寸相加改为无回绕运算；将相接边界误判为不重叠；误用完整地面定点值过滤跨线；把朝向赋值误当成布尔翻转而丢失记录高位；坐标 word 对齐错误覆盖定点小数；把原版只调整源角色的边界修正改为夹紧目标角色；边界差的 TST.W 与有符号溢出比较混淆；不保留多记录／多目标的原循环顺序；漏掉完整动画重置或清除未被原函数写入的字段；形状本身被消费函数错误改写；观察器改变原版帧；把组件调用数当成完整对局。

自然观察比较固定输入全部 3,600 帧 RAM／画面基线。隔离原 CPU 记录显式声明私有输入，覆盖全部过滤条件、朝向、层级、接触／不接触、尺寸和坐标回绕、边界修正、多个源／目标及重复形状。比较完整参与人物、抓取与受击形状、计数及场地边界；不向 Go 注入原版中间输出。

原 A600 生产者容量尚未恢复。本组件使用语义记录切片和已用 byte 偏移，校验 32 bytes 对齐及切片边界，不以猜测的固定容量限制生产逻辑。当前验证范围由具体原 CPU 夹具列出。

## 已验证结果

- 自然完整调用 2,068 次，其中 9 次同时具有抓取与受击记录，实际接触 18 次。全部 3,600 帧 RAM／视频与固定无观察器基线一致。
- 原 CPU 隔离完整调用 278 次，实际接触 359 次。覆盖空缓冲、过滤条件、朝向高位、两个层级、几何回绕、定点小数保留、反常场地边界、24 种四人物排列、重复形状及各 16 条记录的完整缓冲。
- 每次核对参与人物完整 256 bytes、抓取区 512 bytes、受击区 256 bytes、计数和左右边界。隔离执行另检查完整 RAM 的意外写入；仅排除调用栈写入，零意外变化。自然函数观察未改写游戏内存。
- 原生竞态和 Node/Wasm 的两项聚焦测试均通过，保留真实实现前缺类型／函数失败输出。首次全包聚焦命令遇到并行攻击阶段尚缺实现的预期构建失败；实际通过结果使用显式源文件命令，不把当时全包构建描述为通过。

原版将源角色朝向设置为 `+71.bit3` 的反值，目标朝向由记录赋值后按源朝向翻转 bit0；其余目标朝向位保留。对齐只修改坐标高 word，保留源坐标小数。边界修正仅移动源角色，目标保持对齐结果。双方重置动作／姿态和动画字段后，源阶段设置为 `0x20`，目标阶段为零；后续记录继续读取这些实际写回值。

## 复跑

从本 worktree 根目录运行。原 CPU 采样需要已固定的日版 ROM、钩子内核和过滤器；其来源摘要在清单中。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_throw_contacts.py \
  --out .local/md-yuyuhakusho/throw-contacts

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -run '^TestThrowContacts' -count=1 -json \
  $(rg --files -g '*.go' -g '!**/*_test.go' games/yuyuhakusho/engine) \
  games/yuyuhakusho/engine/throw_contacts_test.go

GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  -run '^TestThrowContacts' -count=1 -json \
  $(rg --files -g '*.go' -g '!**/*_test.go' games/yuyuhakusho/engine) \
  games/yuyuhakusho/engine/throw_contacts_test.go
```

夹具和来源绑定见 [验证结果](testdata/throw-contacts-validation.json)，原始 CPU 输入／输出保留在 `testdata/throw-contacts-*.json.gz`。这些是完整消费者调用对照，仍不是平台 E2E 或完整投掷对局；A600 生产、后续动作、伤害和完整碰撞管理器另行验收。
