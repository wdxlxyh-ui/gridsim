# Microgrid v3 — IOA 精准访问 + 自动公式引擎

> **状态**: 实施中 | **日期**: 2026-05-23

## 核心变更

### 1. IOA 精准访问（已实现）

**以前**: `readStoreValue(name)` / `updateStoreValue(name, val)` 遍历 store 按 name 匹配
**现在**: `readPt(name)` / `writePt(name, val)` 通过预建 `pointIOA[name]→IOA` 索引，直接 `store.Get(ioa)` / `store.SetValue(ioa, val)`，O(1)

```
Engine.pointIOA map[string]uint32  // "dev1_Power" → 101
readPt("dev1_Power") → pointIOA["dev1_Power"]=101 → store.Get(101) → 48.5 ✅
writePt("dev1_Power", 50) → store.SetValue(101, 50) ✅
```

### 2. 自动公式生成

**引擎启动时自动创建公式**：

| 公式 | 表达式 | 说明 |
|------|--------|------|
| GRID_P | `{dev1_Power}+{dev2_Power}+...−{pv1_Power}−...` | 用电−发电 |
| 各设备_Power | 引擎直接写入 store | tick() 中计算 |

设备功率计算链路（tick）：
```
PV:    control_mode=remote → AO setpoint(上限ratedPower)
       control_mode=local  → 保持上次值(通过store读)
Load:  control_mode=remote → AO setpoint(上限ratedPower) 
       control_mode=local  → 保持上次值
Bat:   control_mode=remote → AO setpoint(上限ratedPower)
       control_mode=local  → 保持上次值(无时段调度)
```

### 3. 剔除随机计算

移除：
- `irradiance := 300.0 + rand.Float64()*600.0` 
- `dev.Params.LoadRatedKW * (0.5 + rand.Float64()*0.5)`
- 电池时段调度（PV peak/Evening peak/Off-peak 分支）

改为：无 setpoint/strategy 时，从 store 读取当前值（保持不变）。

### 4. GRID 自动公式

`evaluateFormulasLocked()` 已有公式 `GRID_P = {用电之和} - {发电之和}`，自动计算关口功率。

## 实施步骤

### Step 1: 清理 tick() — 移除随机计算

PV: 仅保留 AO/setpoint 分支，移除 irradiance
Load: 仅保留 setpoint 分支，移除随机
Battery: 仅保留 AO setpoint 分支（已改），移除时段调度

### Step 2: 自动添加 GRID 公式

在 engine.Start() 中调用 `e.ensureGridFormula()`，向 `e.topology.Formulas` 添加 GRID_P 公式。

### Step 3: 验证

```bash
go build ./... && go test ./internal/microgrid/ -count=1
npm run build
make dist
```
