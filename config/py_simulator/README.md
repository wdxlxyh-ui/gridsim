# Python 微电网模拟器 — 配置与代码说明

> 最后更新：2026-07-09

---

## 一、项目总览

这是一个纯 Python 实现的微电网设备仿真系统，通过 Modbus TCP 协议对外暴露各设备的运行数据。无任何第三方依赖，仅使用 Python 标准库。

**它做什么**：模拟光伏、储能、充电桩、负荷、电表五种设备的实时运行状态，外部系统（如 EMS、SCADA、GridSim Go 主程序）通过 Modbus TCP 连接读取数据或下发控制指令。

**运行方式**：

```bash
python main.py              # 默认端口 5021
python main.py --port 5022  # 指定端口
```

---

## 二、目录结构一览

```
py_simulator/
│
├── main.py                      ← 主入口（唯一启动文件）
│
├── config/                      ← 所有配置文件
│   ├── device.json              ← ★ 核心配置：设备拓扑与参数
│   ├── modbus_registers.py      ← ★ 寄存器地址映射表
│   ├── logging_config.json      ← 日志配置
│   ├── pv_curve.csv             ← 光伏功率曲线数据
│   ├── load_curve.csv           ← 负荷功率曲线数据
│   └── deviceLogic.conf         ← 预留文件（当前未使用）
│
├── src/
│   ├── models/                  ← 五种设备的物理模型
│   │   ├── pv_model.py          ← 光伏模型
│   │   ├── battery_model.py     ← 储能模型
│   │   ├── ev_model.py          ← 充电桩模型
│   │   ├── load_model.py        ← 负荷模型
│   │   └── meter_model.py       ← 电表模型（功率汇总）
│   └── communication/
│       └── modbus_server.py     ← Modbus TCP Server 实现
│
├── utils/
│   ├── config_loader.py         ← JSON 配置加载工具
│   └── locks.py                 ← 线程锁（全局共享）
│
├── bin/
│   ├── start.sh                 ← Linux 启动脚本
│   └── stop.sh                  ← Linux 停止脚本
│
├── log/                         ← 运行时日志目录（自动创建）
├── build/                       ← PyInstaller 构建产物（可忽略）
├── test/                        ← 测试目录（待补充）
├── py-microgrid-sim.spec        ← PyInstaller 打包配置
├── MicroGridSimulator.service   ← systemd 服务文件
└── README.md                    ← 本文档
```

---

## 三、配置文件详解

### 3.1 device.json — 核心配置（最重要）

这是唯一需要你关心的配置文件。它定义了整个微电网的设备组成和每个设备的参数。

**文件位置**：`config/device.json`

**完整结构**：

```json
{
  "start_time": "14:19",        ← 仿真起始时刻（HH:MM 格式）
  "Devices": [                  ← 设备列表，每种类型一个数组
    { "Meter": [...]  },        ← 电表（通常只有 1 个）
    { "PV": [...]     },        ← 光伏（可多个）
    { "BESS": [...]   },        ← 储能（可多个）
    { "EV": [...]     },        ← 充电桩（可多个）
    { "Load": [...]   }         ← 负荷（可多个）
  ]
}
```

---

#### start_time — 仿真起始时间

| 字段 | 类型 | 示例 | 说明 |
|------|------|------|------|
| start_time | string | `"14:19"` | HH:MM 格式，模拟器从这个时刻开始算起 |

PV 和 Load 的曲线会从这个时刻开始演算。比如设为 `"06:00"`，PV 刚好在日出时开始发电。

---

#### Meter — 电表

```json
{
  "DeviceKey": "Meter_01",    ← 设备唯一标识（随意起名）
  "slave_id": 1               ← Modbus 从站地址（1~247，不可重复）
}
```

电表不需要其他参数，它会自动汇总所有设备的功率。

**汇总逻辑**：`并网功率 = 负荷 + 充电桩 + 储能充电 - 光伏 - 储能放电`

- 正值 = 从电网取电（买电）
- 负值 = 向电网送电（卖电）

---

#### PV — 光伏

```json
{
  "DeviceKey": "PV_01",       ← 设备唯一标识
  "ratedPower": 100,          ← ★ 额定功率 kW（必填）
  "mode": 0,                  ← 出力模式：0=CSV 回放，1=正弦曲线
  "csv_file": "pv_curve.csv", ← mode=0 时，指定 CSV 文件名
  "slave_id": 2               ← Modbus 从站地址
}
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| DeviceKey | ✅ | — | 唯一名称 |
| ratedPower | ✅ | — | 额定功率 kW |
| mode | ❌ | 1 | 0=CSV 回放，1=正弦曲线 |
| csv_file | mode=0 时必填 | "" | CSV 文件名（在 config/ 目录下） |
| slave_id | ❌ | 自动分配 | 1~247 |

**mode=1 正弦模式**：6:00~18:00 按 `sin²(x)` 曲线出力，峰值=ratedPower，叠加 ±5% 随机波动，夜间为 0。

**mode=0 CSV 模式**：从文件读取 24 小时功率数据，线性插值。

---

#### BESS — 储能

```json
{
  "DeviceKey": "BESS_01",
  "ratedCapacity": 12000,       ← ★ 额定容量 kWh（必填关键参数）
  "maxChargePower": 9900,       ← 最大充电功率 kW
  "maxDischargePower": 9900,    ← 最大放电功率 kW
  "socMax": 100,                ← SOC 上限 %（到上限停止充电）
  "socMin": 0,                  ← SOC 下限 %（到下限停止放电）
  "initial_soc": 0.5,           ← 初始 SOC（0~1，0.5=50%）
  "voltage_nominal": 48,        ← 标称电压 V
  "resistance": 0.01,           ← 内阻 Ω
  "slave_id": 5
}
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| DeviceKey | ✅ | — | 唯一名称 |
| ratedCapacity | ❌ | 200 | 电池额定容量 kWh |
| maxChargePower | ❌ | 100 | 最大允许充电功率 kW |
| maxDischargePower | ❌ | 100 | 最大允许放电功率 kW |
| socMax | ❌ | 0.9 | SOC 上限（注意：填百分比数值如 90 或小数如 0.9） |
| socMin | ❌ | 0.1 | SOC 下限 |
| initial_soc | ❌ | 0.5 | 初始 SOC（0~1） |
| voltage_nominal | ❌ | 48 | 标称电压 V |
| resistance | ❌ | 0.01 | 内阻 Ω |
| slave_id | ❌ | 自动分配 | |

**控制方式**：通过 Modbus FC16 写入寄存器偏移 14（`BS.SysAPSetPoint`），正值=充电，负值=放电。

**SOC 计算**：每秒计算一次，`SOC变化 = 功率 × (1/3600) / 容量 × 100%`

---

#### EV — 充电桩

```json
{
  "DeviceKey": "EV_01",
  "PUB_CONN.RatedPW": 22,              ← ★ 额定充电功率 kW（必填）
  "PUB_CONN.MinChargePW": 0,           ← 最小充电功率 kW
  "charge_factor": 1.0,                ← 充电系数（0~1）
  "charge_schedule": [                  ← ★ 充电时段列表
    {"start": "03:30", "stop": "04:30"},
    {"start": "12:00", "stop": "13:00"},
    {"start": "18:00", "stop": "18:30"}
  ],
  "charger_type": "AC",                ← 充电器类型：AC 或 DC
  "phase_mode": 1,                     ← 相模式：1=单相，3=三相
  "voltage": 230.0,                    ← 电压 V
  "power_factor": 0.98,                ← 功率因数
  "slave_id": 4
}
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| DeviceKey | ✅ | — | 唯一名称 |
| PUB_CONN.RatedPW | ✅ | — | 额定功率 kW |
| PUB_CONN.MinChargePW | ❌ | 0 | 最小功率 kW |
| charge_factor | ❌ | 1.0 | 实际功率 = 额定 × 系数 |
| charge_schedule | ❌ | [{"start":"8:00","stop":"10:00"}] | 充电时段 |
| charger_type | ❌ | "AC" | AC 或 DC |
| phase_mode | ❌ | 3 | 1=单相，3=三相（DC 强制单相） |
| voltage | ❌ | AC:220 / DC:400 | 电压 V |
| power_factor | ❌ | AC:0.95 / DC:1.0 | 功率因数 |
| slave_id | ❌ | 自动分配 | |

**充电逻辑**：只在 `charge_schedule` 定义的时段内充电，时段外功率为 0。支持跨零点时段（如 `"start":"23:00","stop":"06:00"`）。

---

#### Load — 负荷

```json
{
  "DeviceKey": "LOAD_001",
  "base_power": 50,             ← 基础功率 kW（mode=1 时使用）
  "mode": 0,                    ← 0=CSV 回放，1=随机波动
  "csv_file": "load_curve.csv", ← mode=0 时的 CSV 文件
  "slave_id": 8
}
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| DeviceKey | ✅ | — | 唯一名称 |
| base_power | ❌ | 120 | 基础功率 kW |
| mode | ❌ | 0 | 0=CSV 回放，1=随机波动 (±20%) |
| csv_file | mode=0 时必填 | "" | CSV 文件名 |
| slave_id | ❌ | 自动分配 | |

---

### 3.2 modbus_registers.py — 寄存器地址映射表

**文件位置**：`config/modbus_registers.py`

**作用**：定义每种设备类型有哪些测点，以及每个测点在 Modbus 寄存器中的偏移地址。

**你通常不需要修改这个文件**，除非要新增测点。

每种设备的寄存器映射一览：

#### Meter 电表寄存器

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| ActivePower | 0 | 并网点有功功率 kW |

#### PV 光伏寄存器

| 测点名 | 偏移地址 | 读/写 | 说明 |
|--------|---------|------|------|
| INV.GenActivePW | 0 | 读 | 当前发电功率 kW |
| INV.APProductionKWH | 2 | 读 | 累计发电量 kWh |
| INV.LimitPower | 4 | **写** | 限功率值 kW |
| INV.OemState | 6 | 读 | OEM 状态 |
| INV.CtrlState | 8 | 读 | 控制状态 |
| INV.Start | 10 | 读 | 启动标志 |
| INV.Stop | 12 | 读 | 停止标志 |
| INV.State | 14 | 读 | 运行状态 |
| INV.APProduction | 16 | 读 | 瞬时发电功率 kW |

#### BESS 储能寄存器

| 测点名 | 偏移地址 | 读/写 | 说明 |
|--------|---------|------|------|
| BS.ActivePW | 0 | 读 | 充放电功率 kW |
| BS.Soc | 2 | 读/**写** | SOC %（可外部设置） |
| BS.MaxChargePower | 4 | 读 | 最大充电功率 |
| BS.MaxDischargePower | 6 | 读 | 最大放电功率 |
| BS.EndChargeSOC | 8 | 读 | 充电截止 SOC |
| BS.EndDischargeSOC | 10 | 读 | 放电截止 SOC |
| BS.Soh | 12 | 读 | SOH % |
| BS.SysAPSetPoint | 14 | **写** | ★ 功率控制指令 kW |
| BS.OemState | 16 | 读 | OEM 状态 |
| BS.CtrlState | 18 | 读 | 控制状态 |
| BS.TotalChargingEng | 20 | 读 | 累计充电量 kWh |
| BS.TotalDischargingEng | 22 | 读 | 累计放电量 kWh |

#### EV 充电桩寄存器

| 测点名 | 偏移地址 | 读/写 | 说明 |
|--------|---------|------|------|
| PUB_CONN.ChargePW | 0 | 读 | 当前充电功率 kW |
| PUB_CONN.CurrentL1 | 2 | 读 | L1 电流 A |
| PUB_CONN.CurrentL2 | 4 | 读 | L2 电流 A |
| PUB_CONN.CurrentL3 | 6 | 读 | L3 电流 A |
| PUB_CONN.ChargePWSet | 8 | **写** | 功率设定 kW |
| PUB_CONN.ChargeCurSetL1 | 10 | **写** | 电流设定 A |
| PUB_CONN.OemState | 12 | 读 | OEM 状态 |
| PUB_CONN.ChargeEnergyKWH | 14 | 读 | 累计充电量 kWh |
| PUB_CONN.CtrlState | 16 | 读 | 控制状态 |
| PUB_CONN.State | 18 | 读 | 运行状态 |
| PUB_CONN.PhaseMode | 20 | **写** | 相模式（1/3） |

#### Load 负荷寄存器

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| Load.Power | 0 | 负荷功率 kW |

**关键规则**：
- 每个测点占 **2 个** 16-bit 寄存器（合计 32-bit 有符号整数）
- 存储值 = 实际物理值 × 100（例：50.25 kW 存储为 5025）
- 偏移地址是寄存器号（不是字节偏移），相邻测点间隔 2

---

### 3.3 logging_config.json — 日志配置

**文件位置**：`config/logging_config.json`

```json
{
  "log_level": "INFO",                  ← 仿真状态日志级别
  "log_dir": "log",                     ← 日志输出目录
  "log_file": "simulation.log",         ← 状态日志文件名
  "max_size": 10,                       ← 单文件最大 MB
  "max_backups": 5,                     ← 保留滚动文件数
  "message_log_file": "message.log",    ← Modbus 报文日志
  "message_log_level": "DEBUG",         ← 报文日志级别
  "traffic_log_file": "modbus_traffic.log"  ← 通信流量日志
}
```

| 参数 | 说明 | 建议值 |
|------|------|--------|
| log_level | 控制 simulation.log 输出量 | 日常用 `INFO`，调试用 `DEBUG` |
| message_log_level | 控制 Modbus 报文日志 | `INFO` 只记关键事件，`DEBUG` 记全部 |
| max_size | 单文件大小上限 | 10 MB |
| max_backups | 保留几个历史文件 | 5 个 |

**三种日志分工**：

| 日志文件 | 记录内容 | 什么时候看 |
|---------|---------|-----------|
| `simulation.log` | 设备状态更新、功率变化、SOC 变化 | 排查设备逻辑问题 |
| `message.log` | Modbus 请求/响应的协议细节 | 排查通信连接问题 |
| `modbus_traffic.log` | 每次读写的寄存器值和耗时 | 排查数据不一致问题 |

---

### 3.4 CSV 功率曲线文件

**文件位置**：`config/pv_curve.csv`、`config/load_curve.csv`

**格式**（无表头，两列）：

```
小时,功率(kW)
0.0,0.0
0.25,10.0
0.5,20.0
...
24.0,0.0
```

| 规则 | 说明 |
|------|------|
| 第一列 | 小时数（0.0 ~ 24.0） |
| 第二列 | 功率值 kW |
| 步长 | 0.25 小时（15 分钟）一个点 |
| 总行数 | 97 行（0.0 到 24.0，含首尾） |
| 中间值 | 自动线性插值 |
| 表头 | 可有可无（有表头会自动跳过） |

**内置曲线说明**：

| 文件 | 特征 |
|------|------|
| `pv_curve.csv` | 6:00 开始出力，12:00 峰值 200kW，18:00 归零 |
| `load_curve.csv` | 凌晨低谷 20kW，9:00~14:00 高峰 120kW，晚间回落 |

**自定义曲线**：复制一份 CSV 改数据即可，在 device.json 里 `csv_file` 字段指向你的文件名。

---

### 3.5 deviceLogic.conf — 预留文件

当前为空文件，未使用。保留供未来扩展设备联动逻辑。

---

## 四、运行原理

### 4.1 启动流程

```
main.py 启动
    │
    ├─ 1. 加载 config/logging_config.json → 初始化三路日志
    │
    ├─ 2. 加载 config/device.json → 解析设备列表
    │
    ├─ 3. 为每个设备创建 model 实例
    │     ├─ PV → PVModel(config, start_hour)
    │     ├─ BESS → BatteryModel(config, start_hour, ...)
    │     ├─ EV → EVModel(config, start_hour, ...)
    │     ├─ Load → LoadModel(config, start_hour)
    │     └─ Meter → MeterModel(config, devices_data, data_lock, start_hour)
    │
    ├─ 4. 启动 Modbus TCP Server 线程（监听端口）
    │
    └─ 5. 启动数据更新线程（每秒循环）
```

### 4.2 每秒更新循环

```
update_device_data() 每 1 秒执行一次：
    │
    ├─ 计算当前仿真时刻
    │     current_hour = start_hour + (已运行秒数 / 3600)
    │
    ├─ 依次更新非 Meter 设备
    │     ├─ PV: model.update(p_limit=寄存器4的值)
    │     ├─ BESS: model.update(p_command=寄存器14的值)
    │     ├─ EV: model.update(devices_data, device_key)
    │     └─ Load: model.update()
    │
    ├─ 将 model 返回的 dict 按 modbus_registers 映射写入共享数据
    │
    └─ 最后更新 Meter（汇总所有设备的功率）
```

### 4.3 数据流

```
┌────────────────────────────────────────────────────────────┐
│                    Python 进程内部                           │
│                                                            │
│  devices_data (共享字典，data_lock 保护)                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ "PV_01": {slave_id:2, data:{0:80.5, 2:123.4, ...}} │   │
│  │ "BESS_01": {slave_id:5, data:{0:-50, 2:65.2, ...}} │   │
│  │ "Meter_01": {slave_id:1, data:{0:30.5}}            │   │
│  │ ...                                                  │   │
│  └─────────────────────────────────────────────────────┘   │
│       ↑ 每秒写入                    ↓ 按 slave_id 查找      │
│  ┌─────────┐                  ┌────────────────┐           │
│  │ Models  │                  │ Modbus Server  │           │
│  │(计算逻辑)│                  │ (监听 TCP)     │           │
│  └─────────┘                  └───────┬────────┘           │
│                                       │                    │
└───────────────────────────────────────│────────────────────┘
                                        │ Modbus TCP
                                        ↓
                              ┌─────────────────────┐
                              │ 外部系统（EMS/Go）   │
                              │ FC03 读 / FC16 写   │
                              └─────────────────────┘
```

### 4.4 Modbus 通信规则

| 项目 | 值 |
|------|-----|
| 协议 | Modbus TCP (MBAP Header + PDU) |
| 默认端口 | 5021 |
| 数据编码 | 32-bit 有符号整数 (Big-Endian) |
| 系数 | 实际值 × 100 |
| 每测点占用 | 2 个 Holding Register |
| 支持功能码 | FC03（读）、FC16（批量写） |
| 不支持 | FC06（单寄存器写，因为 32 位值需要 2 个寄存器） |

**读取示例**：
- 读 BESS_01 的 SOC → 发送 FC03，Slave ID=5，起始地址=2，数量=2
- 返回 4 字节 → 解析为 int32 → 除以 100 = SOC 百分比

**写入示例**：
- 设置 BESS_01 放电 50kW → FC16，Slave ID=5，起始地址=14，数量=2，值=5000（50×100）
- 设置 BESS_01 充电 30kW → FC16，值=-3000（-30×100，负值=放电方向相反）

---

## 五、设备控制速查

### 5.1 如何控制储能充放电

通过 Modbus 写入 `BS.SysAPSetPoint`（寄存器偏移 14）：

| 写入值 | 含义 |
|--------|------|
| 正值（如 5000） | 充电 50 kW |
| 负值（如 -3000） | 放电 30 kW |
| 0 | 停止充放电 |

SOC 到上限自动停止充电，到下限自动停止放电。

### 5.2 如何限制光伏出力

通过 Modbus 写入 `INV.LimitPower`（寄存器偏移 4）：

| 写入值 | 含义 |
|--------|------|
| 8000 | 限制最大出力 80 kW |
| 0 或不写 | 不限制（使用额定功率） |

### 5.3 如何控制充电桩

两种控制方式（电流优先）：

| 方式 | 寄存器 | 偏移 | 说明 |
|------|--------|------|------|
| 功率控制 | PUB_CONN.ChargePWSet | 8 | 最大充电功率 kW |
| 电流控制 | PUB_CONN.ChargeCurSetL1 | 10 | L1 相电流设定 A（优先级更高） |

---

## 六、slave_id 分配规则

| 规则 | 说明 |
|------|------|
| 范围 | 1 ~ 247 |
| 唯一性 | 同一实例内不可重复 |
| 配置方式 | device.json 中指定 `"slave_id": N` |
| 自动分配 | 不指定则从 1 开始自动递增 |
| 冲突处理 | 检测到重复自动跳过，日志报警 |

**建议规划**：

| 设备类型 | 建议范围 | 示例 |
|---------|---------|------|
| Meter | 1 | slave_id: 1 |
| PV | 2~10 | slave_id: 2, 3, 4... |
| BESS | 11~20 | slave_id: 11, 12... |
| EV | 21~40 | slave_id: 21, 22... |
| Load | 41~60 | slave_id: 41, 42... |

---

## 七、时间仿真机制

| 概念 | 说明 |
|------|------|
| start_time | device.json 中配置的起始时刻 |
| start_hour | 转换为小时数（如 "14:19" → 14.317） |
| 当前仿真时刻 | `start_hour + (已运行秒数 / 3600)` |
| 跨零点 | 自动 `% 24` 处理 |
| 仿真速度 | 1:1 实时（1 秒现实 = 1 秒仿真） |

**示例**：
- 配置 `start_time: "06:00"`
- 程序启动后运行 3600 秒（1 小时）
- 当前仿真时刻 = 7:00
- PV 正好在出力上升期

---

## 八、新增/修改设备的操作步骤

### 8.1 添加一个新的 PV 设备

1. 编辑 `config/device.json`
2. 在 `"PV": [...]` 数组中追加：

```json
{
  "DeviceKey": "PV_02",
  "ratedPower": 200,
  "mode": 1,
  "slave_id": 3
}
```

3. 重启模拟器

### 8.2 修改 BESS 的 SOC 限值

1. 编辑 `config/device.json`
2. 修改 BESS 设备的 `socMax` / `socMin`
3. 重启模拟器

### 8.3 添加自定义功率曲线

1. 创建 CSV 文件放到 `config/` 目录（如 `my_load.csv`）
2. 格式：每行 `小时,功率`，步长 0.25
3. 在 device.json 中引用：`"csv_file": "my_load.csv"`
4. 重启模拟器

---

## 九、源码文件说明

### 9.1 main.py — 主入口

| 函数 | 作用 |
|------|------|
| `load_config()` | 读取 device.json，为每个设备创建 model 实例，初始化 devices_data 字典 |
| `update_device_data()` | 每秒循环：调用各 model.update()，写入寄存器数据 |
| `get_cli_port()` | 解析命令行 --port 参数 |
| `main()` | 启动 Modbus Server 线程 + 数据更新线程 |

### 9.2 src/models/ — 设备模型

| 文件 | 类 | 核心方法 |
|------|----|---------|
| `pv_model.py` | PVModel | `update(p_limit)` → 返回 9 个测点值 |
| `battery_model.py` | BatteryModel | `update(p_command)` → 返回 12 个测点值 |
| `ev_model.py` | EVModel | `update(devices_data, key)` → 返回 12 个测点值 |
| `load_model.py` | LoadModel | `update()` → 返回 1 个测点值 |
| `meter_model.py` | MeterModel | `update()` → 返回 1 个测点值（汇总） |

每个 model 的 `update()` 返回一个 dict，key 是 `modbus_registers.py` 中定义的测点名，value 是物理值。main.py 负责把这个 dict 按寄存器偏移地址写入 `devices_data[key]['data']`。

### 9.3 src/communication/modbus_server.py — 通信层

| 方法 | 作用 |
|------|------|
| `run()` | 绑定 TCP 端口，accept 连接 |
| `handle_connection()` | 逐帧解析 Modbus TCP 包 |
| `process_request()` | 根据 function_code 分发处理 |
| FC03 处理 | 从 devices_data 读寄存器，乘 100 打包返回 |
| FC16 处理 | 解析写入数据，除以 100 写入 devices_data |

### 9.4 utils/ — 工具

| 文件 | 作用 |
|------|------|
| `config_loader.py` | 一个函数：读 JSON 文件返回 dict |
| `locks.py` | 一个全局锁：`data_lock = threading.Lock()` |

---

## 十、常见问题

### Q: 为什么 PV 功率一直是 0？

A: 检查仿真时刻是否在夜间。`start_time` 如果设为 `"20:00"`，PV 在 18:00 以后不发电。改为 `"08:00"` 试试。

### Q: BESS 功率指令不响应？

A: 确认写入的寄存器偏移地址是 14（不是 0），Slave ID 正确，且写入量为 2 个寄存器（FC16，quantity=2）。

### Q: 改了 device.json 没效果？

A: 必须重启模拟器。运行中不支持热加载配置。

### Q: 端口被占用？

A: 换端口 `python main.py --port 5022`，或检查是否有残留进程。

### Q: 多个设备的 slave_id 重复了？

A: 程序启动时会检测并自动跳过重复的，但日志中会打印 error。建议手动规划不重复。

### Q: CSV 数据不到 97 行？

A: 没关系，会用线性插值补全。但至少要有 2 个以上数据点。

### Q: 怎么知道当前仿真走到了几点？

A: 看 `simulation.log`，每秒都会打印 `Current hour = X.XXh`。

---

## 十一、与 GridSim Go 主程序的关系

```
┌─────────────────────────────────────────────────────────────┐
│ GridSim Go 主程序                                           │
│  ├─ Web UI (Vue3)                                          │
│  ├─ REST API                                               │
│  ├─ IEC104 Server                                          │
│  └─ modbus_bridge 适配器                                    │
│       │                                                     │
│       │ 每 1 秒轮询 Modbus TCP (FC03 读取所有寄存器)         │
│       │ 控制指令时通过 FC16 写入                             │
│       ↓                                                     │
│  ┌─────────────────────────────────────┐                    │
│  │ Python 微电网模拟器 (本项目)         │                    │
│  │ 独立进程，监听 Modbus TCP 端口       │                    │
│  └─────────────────────────────────────┘                    │
└─────────────────────────────────────────────────────────────┘
```

Go 主程序负责：
1. 管理 Python 进程的生命周期（启动/停止）
2. 每秒轮询 Modbus 读取数据 → 映射为 IEC104 点位
3. 接收 IEC104 控制指令 → 转发为 Modbus FC16 写入 Python
4. 提供 Web UI 和 REST API

Python 模拟器负责：
1. 物理模型仿真计算
2. 通过 Modbus TCP 暴露数据
3. 接收 Modbus 写入的控制指令

两者唯一的通信方式是 **Modbus TCP**，完全解耦。Python 模拟器也可以脱离 Go 程序独立运行，用任何 Modbus 客户端工具连接。
