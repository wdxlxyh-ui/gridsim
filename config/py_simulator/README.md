# MicroGrid Simulator（微电网模拟器）

基于 Python 的微电网设备仿真系统，通过 Modbus TCP 协议对外暴露各设备的运行数据，支持光伏、储能、充电桩、负荷、电表等设备的实时仿真，可用于微电网 EMS/SCADA 系统的联调测试。

## 功能概览

| 功能模块 | 说明 |
|---------|------|
| 光伏模拟 (PV) | 正弦曲线或 CSV 功率曲线回放，支持限功率控制 |
| 储能模拟 (BESS) | SOC 充放电计算、功率指令响应、电量统计 |
| 充电桩模拟 (EV) | 定时充电调度、AC/DC 充电模式、电流/功率双控制 |
| 负荷模拟 (Load) | CSV 曲线回放或基础功率随机波动 |
| 电表模拟 (Meter) | 自动汇总各设备功率，计算并网点净功率 |
| Modbus TCP 通信 | 标准 Modbus TCP Server，支持 FC03 读寄存器、FC16 写多寄存器 |
| 时间仿真 | 支持自定义仿真起始时间，1 秒步长推进 |
| 日志系统 | 状态日志 + Modbus 报文日志 + 通信流量日志，支持滚动归档 |

## 项目结构

```
microgridsimulate/
├── main.py                         # 主入口：加载配置、启动仿真循环和 Modbus Server
├── MicroGridSimulator.service      # systemd 服务文件（Linux 部署用）
├── bin/
│   ├── start.sh                    # 启动脚本
│   └── stop.sh                     # 停止脚本
├── config/
│   ├── device.json                 # 设备拓扑与参数配置
│   ├── modbus_registers.py         # Modbus 寄存器地址映射表
│   ├── logging_config.json         # 日志配置
│   ├── pv_curve.csv                # 光伏功率曲线（15分钟分辨率）
│   └── load_curve.csv              # 负荷功率曲线（15分钟分辨率）
├── src/
│   ├── communication/
│   │   └── modbus_server.py        # Modbus TCP Server 实现
│   └── models/
│       ├── pv_model.py             # 光伏模型
│       ├── battery_model.py        # 储能模型
│       ├── ev_model.py             # 充电桩模型
│       ├── load_model.py           # 负荷模型
│       └── meter_model.py          # 电表模型（功率汇总）
├── utils/
│   ├── config_loader.py            # JSON 配置加载工具
│   └── locks.py                    # 线程锁
├── log/                            # 运行时日志目录
└── test/                           # 测试目录（待补充）
```

## 设备模型详解

### 1. 光伏模型 (PVModel)

**功能：**
- 两种出力模式：
  - `mode=0`：CSV 曲线回放，从文件中读取 24 小时功率曲线，线性插值
  - `mode=1`：正弦曲线模拟，6:00-18:00 按 sin²(x) 曲线自动生成，叠加 ±5% 随机波动
- 支持限功率控制（通过 Modbus 写入 LimitPower 寄存器）
- 自动累计发电量统计（kWh）
- 输出测点：发电功率、限功率值、累计电量、设备状态等

**关键参数：**
- `ratedPower`：额定功率 (kW)
- `mode`：出力模式 (0=CSV, 1=正弦)
- `csv_file`：CSV 曲线文件名

**Modbus 寄存器：**

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| INV.GenActivePW | 0 | 当前发电功率 (kW) |
| INV.APProductionKWH | 2 | 累计发电量 (kWh) |
| INV.LimitPower | 4 | 限功率值 (kW)，可写入 |
| INV.OemState | 6 | OEM 状态 |
| INV.CtrlState | 8 | 控制状态 |
| INV.Start | 10 | 启动标志 |
| INV.Stop | 12 | 停止标志 |
| INV.State | 14 | 运行状态 |
| INV.APProduction | 16 | 瞬时发电功率 (kW) |

### 2. 储能模型 (BatteryModel)

**功能：**
- SOC 实时计算（基于充放电功率和额定容量）
- 支持功率指令控制（正值充电、负值放电）
- SOC 上下限保护（到限值后自动截止）
- 累计充放电电量统计
- 支持外部直接设置 SOC（通过 Modbus 写入）

**关键参数：**
- `ratedCapacity`：额定容量 (kWh)
- `maxChargePower`：最大充电功率 (kW)
- `maxDischargePower`：最大放电功率 (kW)
- `socMax` / `socMin`：SOC 上下限
- `initial_soc`：初始 SOC（0~1）

**Modbus 寄存器：**

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| BS.ActivePW | 0 | 当前有功功率 (kW) |
| BS.Soc | 2 | SOC (%)，可读可写 |
| BS.MaxChargePower | 4 | 最大充电功率 |
| BS.MaxDischargePower | 6 | 最大放电功率 |
| BS.EndChargeSOC | 8 | 充电截止 SOC |
| BS.EndDischargeSOC | 10 | 放电截止 SOC |
| BS.Soh | 12 | SOH (%) |
| BS.SysAPSetPoint | 14 | 功率设定值，可写入 |
| BS.OemState | 16 | OEM 状态 |
| BS.CtrlState | 18 | 控制状态 |
| BS.TotalChargingEng | 20 | 累计充电电量 (kWh) |
| BS.TotalDischargingEng | 22 | 累计放电电量 (kWh) |

### 3. 充电桩模型 (EVModel)

**功能：**
- 定时充电调度：支持多时段配置，跨零点时段自动处理
- AC/DC 双充电模式：
  - AC 模式：支持单相/三相切换，功率因数计算
  - DC 模式：固定单相，电压范围 300~500V
- 双控制方式：
  - 电流控制优先（ChargeCurSetL1 > 0 时生效）
  - 功率控制兜底（通过 ChargePWSet 设定）
- 自动计算三相电流（L1/L2/L3）
- 累计充电电量统计

**关键参数：**
- `PUB_CONN.RatedPW`：额定充电功率 (kW)
- `PUB_CONN.MinChargePW`：最小充电功率 (kW)
- `charge_factor`：充电系数
- `charge_schedule`：充电时段列表
- `charger_type`：充电器类型 (AC/DC)
- `phase_mode`：相模式 (1=单相, 3=三相)
- `voltage`：额定电压 (V)
- `power_factor`：功率因数

**Modbus 寄存器：**

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| PUB_CONN.ChargePW | 0 | 当前充电功率 (kW) |
| PUB_CONN.CurrentL1 | 2 | L1 相电流 (A) |
| PUB_CONN.CurrentL2 | 4 | L2 相电流 (A) |
| PUB_CONN.CurrentL3 | 6 | L3 相电流 (A) |
| PUB_CONN.ChargePWSet | 8 | 功率设定值，可写入 |
| PUB_CONN.ChargeCurSetL1 | 10 | 电流设定值，可写入 |
| PUB_CONN.OemState | 12 | OEM 状态 |
| PUB_CONN.ChargeEnergyKWH | 14 | 累计充电电量 (kWh) |
| PUB_CONN.CtrlState | 16 | 控制状态 |
| PUB_CONN.State | 18 | 运行状态 |
| PUB_CONN.PhaseMode | 20 | 相模式，可写入 |
| PUB_CONN.Voltage | 22 | 电压 (V×100)，可写入 |

### 4. 负荷模型 (LoadModel)

**功能：**
- 两种模式：
  - `mode=0`：CSV 曲线回放，线性插值
  - `mode=1`：基础功率 ±20% 随机波动
- 15 分钟分辨率功率曲线

**关键参数：**
- `base_power`：基础功率 (kW)
- `mode`：出力模式 (0=CSV, 1=随机)
- `csv_file`：CSV 曲线文件名

**Modbus 寄存器：**

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| Load.Power | 0 | 当前负荷功率 (kW) |

### 5. 电表模型 (MeterModel)

**功能：**
- 自动汇总所有设备功率
- 计算并网点净功率：`净功率 = 负荷功率 - 光伏发电 - 储能放电 + 储能充电`
- 正值表示净消耗（从电网取电），负值表示净发电（向电网送电）

**Modbus 寄存器：**

| 测点名 | 偏移地址 | 说明 |
|--------|---------|------|
| ActivePower | 0 | 并网点有功功率 (kW) |

## Modbus TCP 通信

### 协议说明

- 监听端口：**5021**
- 协议版本：Modbus TCP（MBAP Header + PDU）
- 数据编码：每个测点占 **2 个寄存器**（32 位有符号整数），值 = 实际值 × 100
- Slave ID：每个设备独立分配，支持配置文件指定或自动分配（1~247）

### 支持的功能码

| 功能码 | 名称 | 说明 |
|--------|------|------|
| FC03 | Read Holding Registers | 批量读寄存器（数量必须为偶数，最大 125） |
| FC16 | Write Multiple Registers | 批量写寄存器（数量必须为偶数） |
| FC06 | Write Single Register | 不支持（32位值需使用 FC16） |

### 错误码

| 异常码 | 含义 |
|--------|------|
| 0x01 | 不支持的功能码 |
| 0x02 | 非法地址 / 未找到 Slave ID |
| 0x03 | 非法数据值 |
| 0x04 | 服务端处理异常 |

## 配置说明

### device.json

```json
{
  "start_time": "14:19",       // 仿真起始时间（HH:MM）
  "Devices": [
    { "Meter": [{ "DeviceKey": "Meter_01", "slave_id": 1 }] },
    { "PV": [{ "DeviceKey": "PV_01", "ratedPower": 100, "mode": 0, "csv_file": "pv_curve.csv", "slave_id": 2 }] },
    { "BESS": [{ "DeviceKey": "BESS_01", "ratedCapacity": 12000, "maxChargePower": 9900, ... }] },
    { "EV": [{ "DeviceKey": "EV_01", "PUB_CONN.RatedPW": 22, "charger_type": "AC", ... }] },
    { "Load": [{ "DeviceKey": "LOAD_001", "base_power": 50, "mode": 0, "csv_file": "load_curve.csv" }] }
  ]
}
```

### CSV 功率曲线格式

每行两列，无表头（或首行可跳过）：
```
小时(0~24), 功率(kW)
0.0,0.0
0.25,10.0
...
```

步长为 0.25 小时（15 分钟），中间值线性插值。

### logging_config.json

```json
{
  "log_level": "INFO",            // 状态日志级别
  "log_dir": "log",               // 日志目录
  "log_file": "simulation.log",   // 状态日志文件
  "max_size": 10,                 // 单文件最大 MB
  "max_backups": 5,               // 保留文件数
  "message_log_file": "message.log",       // Modbus 报文日志
  "message_log_level": "DEBUG",
  "traffic_log_file": "modbus_traffic.log" // 通信流量日志
}
```

## 部署与运行

### 环境要求

- Python 3.6+
- 无第三方依赖（纯标准库实现）

### 直接运行

```bash
python main.py
```

### Linux systemd 服务部署

```bash
# 拷贝服务文件
cp MicroGridSimulator.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable MicroGridSimulator
systemctl start MicroGridSimulator
```

### 启停脚本

```bash
./bin/start.sh   # 启动服务
./bin/stop.sh    # 停止服务
```

## 时间仿真机制

- 仿真起始时间通过 `device.json` 中的 `start_time` 字段配置
- 程序启动后以实时时钟推进（1 秒 = 1 秒）
- 各设备模型根据 `当前仿真小时 = start_hour + elapsed_seconds / 3600` 计算出力
- 支持跨零点自动处理（`current_hour % 24`）

## 技术特点

- **纯 Python 标准库实现**：无需安装任何第三方包
- **多设备并行仿真**：单进程多线程架构，线程安全的数据共享
- **灵活配置**：设备数量、参数、Slave ID 均通过 JSON 配置
- **日志分离**：状态日志、报文日志、流量日志独立文件，支持滚动归档
- **Modbus 标准协议**：兼容标准 Modbus TCP 客户端（如 ModbusPoll、EMS 系统）

## 已知限制

- 仿真步长固定为 1 秒，不支持加速/减速
- 不支持设备动态增删（需重启）
- FC06 单寄存器写入不支持（32 位值必须使用 FC16）
- 仅支持 Holding Registers（不支持 Coils/Input Registers）
- 无 Web UI，纯命令行运行
