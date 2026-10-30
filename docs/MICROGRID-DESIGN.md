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

---

## 8. 一体化调度仿真扩展 (v2.6)

面向"调度系统 → 风/光/储 EMS → 设备 → 并网点"的有功功率闭环仿真，只模拟有功，不含无功/电压/频率控制。所有新行为默认关闭，旧拓扑的点表与行为不变。

### 8.1 新增设备类型

| 类型 | 前缀 | 说明 |
|------|------|------|
| `wind` | 风电N | 风机。理论功率由 `_风速` 测点经立方曲线计算(切入 3 / 额定 12 / 切出 25 m/s 可配)，出力 = min(设定值, 理论功率) |
| `ems` | 风电EMS/光伏EMS/储能EMS N | 站级控制器，`ems_kind` = wind/pv/storage，默认管理拓扑内全部同类设备，可用 `ems_members` 限定 |

### 8.2 设备动态参数 (`params`)

| 字段 | 含义 |
|------|------|
| `ramp_up_kw_per_s` / `ramp_down_kw_per_s` | 爬坡率；只配一个时两个方向共用；0 = 不限速 |
| `response_delay_ms` | 指令延迟生效 |
| `available_mode: "limit"` | 光伏出力 = min(设定值, `_理论功率` 测点)；`_理论功率` 可被自动变化策略/CSV 驱动 |
| `soc_protection` | 储能到 SOC 限值时在功率层禁止继续充/放，并生成 `_最大可充功率/_最大可放功率` 测点 |
| `zero_is_command` | 远方设定值 0 作为真实 0 指令 |

**置数语义保持不变**：远方模式下设定值为 0 时读取 `_Power`，因此直接把功率点写 0 会立即生效，且绕过爬坡与延迟(引擎检测到功率点被外部改写时丢弃排队中的旧指令)。EMS 下发的指令例外：EMS 站级设定 0 是真实 0 指令。

### 8.3 EMS 分配

- 风/光 EMS：站级设定按各单元可用功率比例拆分，封顶于可用功率之和。
- 储能 EMS：按各单元在该方向上的可充/可放余量(受 SOC 保护)比例拆分，正充负放。
- EMS 本地模式(`_远方控制`=0)或通讯中断(`_通讯状态`=0)时不下发，单元保持上一次指令。
- 汇总测点：总有功、总额定、可调上限/下限、远方状态、储能 EMS 的容量加权 SOC。

### 8.4 并网点

- `grid_meter.import_limit_kw` / `export_limit_kw`：独立的进口/出口限值，0 = 沿用 `rated_capacity_kw`。
- `grid_meter.breaker_points`：生成 `关口表_并网开关状态`(DI 1003)与 `关口表_并网开关遥控`(DO 1011)；遥控分闸即孤岛，并网点功率为 0。关口表遥信/遥控区 1001~1050 不参与设备 IOA 自动分配。

### 8.5 Modbus TCP

`microgrid_config.modbus_port` > 0 时，在 IEC104 之外再起一个 Modbus 监听，共用同一份点表：

| 点类型 | 功能码 | 地址 |
|------|------|------|
| AI | FC04 | IOA×2 (float32，2 个寄存器) |
| AO | FC03 读 / FC06,16 写 | IOA×2 |
| DI | FC02 | IOA |
| DO | FC01 读 / FC05,15 写 | IOA |

字节序 `byte_order`：ABCD(默认)/BADC/CDAB/DCBA；`modbus_slave_id` 默认 1。测量点(AI)经 Modbus 只读；仿真输入(如风速)经 REST 置数。热加载新增的设备在重启实例后才有 Modbus 地址。

### 8.6 测试

- `dynamics_test.go`、`ems_test.go`、`grid_test.go`：引擎行为。
- `modbus_e2e_test.go`：原始 TCP 帧的闭环(下发 → tick → 读回)、字节序、线圈、地址唯一性。
