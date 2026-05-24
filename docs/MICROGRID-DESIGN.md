# 微电网仿真模块 — 完整设计方案 v2.5

> 状态: 已实现 | 审查日期: 2026-05-24

---

## 1. 架构总览

```
┌─────────────────────────────────────────────────────┐
│ MicrogridEditor.vue (Vue 3 + Element Plus)         │
│  设备管理 / SVG拓扑 / 仪表盘 / 测点策略 / 导出       │
├─────────────────────────────────────────────────────┤
│ internal/microgrid/                                 │
│   model.go     → 数据模型                           │
│   engine.go    → 仿真引擎 (tick/功率平衡/SOC/IOA)    │
│   pointmap.go  → IEC104 测点展开 (EGC兼容命名)      │
│   handler.go   → REST API (6组端点)                 │
│   history.go   → 历史快照                          │
├─────────────────────────────────────────────────────┤
│ pkg/library/store.go → 并发安全内存点表              │
│ internal/detail/     → auto-change 策略引擎          │
│ internal/manager/    → 实例生命周期                  │
└─────────────────────────────────────────────────────┘
```

---

## 2. 数据模型

### 2.1 Topology

```go
type Topology struct {
    GridMeter    GridMeterConfig   // 关口表
    BusName      string            // 母线名称
    BusVoltageKV float64           // 母线电压(kV)
    Devices      []Device          // 设备列表
    Formulas     []FormulaRule     // 自定义公式
}
```

### 2.2 Device

```go
type Device struct {
    ID           string          // 唯一标识 "dev-1"
    Type         ComponentType   // pv/battery/load/charger
    Name         string          // 用户命名
    Switch       DeviceSwitch    // 开关状态
    Params       DeviceParams    // 类型专属参数
    ControlMode  ControlMode     // remote(远方)/local(本地)
    Strategy     *StrategyConfig // 本地策略
    CustomPoints []CustomPoint   // 自定义测点
}
```

### 2.3 IOA 分配

```
关口表:      AI 1~4, DI 1001~1002
设备 i(从0): AI 101+i×50+0~4, DI 1101+i×50, AO 3001+i×50, DO 4001+i×50
自定义测点: 在每个设备段末尾递增
```

---

## 3. 仿真引擎

### 3.1 Tick 循环

```
每周期:
  1. PV 功率 ← AO setpoint(remote) 或 store值(local)
  2. 负荷/充电桩 ← store当前值
  3. 储能 ← AO setpoint(remote) 或 store值(local)
  3.5 SOC 更新 ← ΔSOC = power×dt/capacity×100
  4. 功率平衡 ← GRID = Load + Charger + Battery − PV
  5. syncStoreLocked ← 写入 _Power/_SOC/_SwStatus/GRID_P 等
  5.5 公式评估 ← evaluateFormulasLocked (自动GRID公式)
  6. 快照记录
```

### 3.2 IOA 索引

```
buildPointIndex():
  1. 扫描store所有点 Name→IOA
  2. 加载持久化PointsJSON
  3. 为每个设备创建 dev.ID 别名 (engine内部用)
```

### 3.3 功率约定

| 量 | + 含义 | − 含义 |
|----|--------|--------|
| PV | 发电 | — |
| Battery | 充电 | 放电 |
| Load/Charger | 用电 | — |
| GRID | 从电网用电 | 向电网送电 |

---

## 4. API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/microgrid/{id}/topology` | GET/PUT | 拓扑读写 |
| `/microgrid/{id}/device` | POST/PUT/DELETE | 设备CRUD |
| `/microgrid/{id}/control/{devId}` | POST | 开关控制 |
| `/microgrid/{id}/dashboard` | GET | 实时数据(设备级数组) |
| `/microgrid/{id}/points` | GET | 测点列表(sorted+can_toggle+local_mode) |
| `/microgrid/{id}/formulas` | GET/POST/PUT/DELETE | 公式CRUD |
| `/microgrid/{id}/export-xlsx` | GET | 导出EGC兼容xlsx |

---

## 5. EGC 兼容命名

| 设备 | 测点名格式 | Alias (EGC标识符) |
|------|-----------|------------------|
| 关口表 | 关口表_有功功率 | METER.ActivePW |
| 光伏 | 光伏{N}_有功功率 | INV.GenActivePW |
| 储能 | 储能{N}_充放电功率 | BS.ActivePW |
| 负荷 | 负荷{N}_有功功率 | LOAD.ActivePW |
| 充电桩 | 充电桩{N}_充电功率 | PUB_CONN.ChargePW |

---

## 6. 前端组件

### 6.1 拓扑 Tab
- 关口表/母线/设备配置表单
- SVG 拓扑图 (固定viewBox 680×450, width:100%)
- 功率流向动画 (fl-up/fl-dn/fz)
- 公式预览 (autoFormulas)

### 6.2 测点 Tab
- IOA/类型/名称/当前值/控制模式(开关)/策略
- 本地模式 → 配置策略(11种)
- 2s 自动刷新(对比式更新,无频闪)

### 6.3 仪表盘
- 三栏: 并网点/光伏总/负荷总
- 设备级功率列表 (实时值,从store读取)

---

## 7. 关键设计决策

| 决策 | 说明 |
|------|------|
| 名称用全局索引 | 光伏1/储能2/负荷3 (非按类型) |
| engine存dev.ID别名 | buildPointIndex双索引(中文名+dev.ID) |
| 策略持久化 | auto-change engine驱动,local_mode从active策略读取 |
| SaveConfigOnly | 保存拓扑不停止实例 |
| ReloadTopology | 热更新拓扑到运行中引擎 |
| 无随机生成 | PV/负荷/储能仅从store读,无irradiance/random |
