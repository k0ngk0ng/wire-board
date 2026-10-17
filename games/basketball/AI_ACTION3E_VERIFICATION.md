# 原版 Action3E 的电脑分派

## 实现前失败场景

- Court2混合video10280／logic3424、全电脑video13301／logic4418的实际前态都是Action3E持球、CarriedPlayer=-1、未依附。不能依据旧错误文案把3E局限为搭载玩家。
- 原A05D先按95EF[mode]&0x10判断是否直接分派，mode1／29／31／34允许；不读CarriedPlayer。其他mode只有当前人物持球才重置为mode1，否则原样返回。
- mode1的A11E在持球且Action3E时设置mode31、不立即执行31；无球转mode0。Entered、请求、动作和RNG须保持原写入边界。
- 当前mode超出已恢复flag表必须报错，不默认判为可跳过或随意重置。
- 独立原入口gate执行与完整mode1执行先作对照，覆盖是否持球、进入位、模式、席位以及不相关CarriedPlayer值；不以通过入口判断宣称后续3E物理动作已恢复。

## 已执行验证

- REA 的 bank5 A05D 分派分析保留于 `.local/basketball/rea/ai-batch/batch.json`，证据 ID：`ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d`。
- `verify_ai_action3e.py` 使用未修改的原 ROM 指令，生成 2,016 个 A05D 分派入口和 288 个完整 A05D→A11E 调用。包含全部 42 个模式、4 个席位、持球与进入位组合。
- `TestOriginalAction3EDispatchAndPossession` 对照原版结果通过。夹具及摘要位于 `engine/testdata/ai-action3e-function*`。
- 首次修复后两个固定 Court2 完整验收推进到同一帧的 Action62 物理边界；并非通过该对局。后续 Action62 和 mode31 的验证分别记录在相应文档。

复现：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_action3e.py
source scripts/env.sh
go test ./games/basketball/engine -run '^TestOriginalAction3EDispatchAndPossession$' -count=1 -v
```

该矩阵是隔离函数验证，不是自然原版完整比赛的证明。
