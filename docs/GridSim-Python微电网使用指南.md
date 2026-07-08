# GridSim — Python 微电网实例使用指南

本文档说明如何在 GridSim 中创建、配置、运行 Python 微电网仿真实例，包括多实例管理和设备控制。

## 1. 概述

Python 微电网实例基于物理模型仿真器，每个实例独立运行一个 Python 进程，通过 Modbus TCP 协议对外暴露设备数据。支持以下设备类型：

| 设备类型 | 缩写 | 功能 |
|---------|------|------|
| 电表 (Meter) | Meter | 自动汇总所有设备功率，计算并网点净功率 |
| 光伏 (PV) | PV | 正弦曲线或 CSV 功率曲线回放，支持限功率控制 |
| 储能 (BESS) | BESS | SOC 充放电计算、功率指令响应、电量统计 |
| 充电桩 (EV) | EV | AC/DC 双模式、单相/三相、定时充电调度 |
| 负荷 (Load) | Load | CSV 曲线回放或基础功率随机波动 |

### 1.1 多实例架构

每个 Python 微电网实例拥有：
- 独立的 Modbus TCP 端口（如 5021、5022、5023...）
- 独立的工作目录（`config/py_instances/<instance-id>/`）
- 独立的设备配置文件（`config/device.json`）
- 独立的日志目录（`log/`）
- 独立的 Python 进程

```
Go 主程序
├── 实例A → python3 main.py --port 5021  (工作目录: py_instances/abc123/)
├── 实例B → python3 main.py --port 5022  (工作目录: py_instances/def456/)
└── 实例C → python3 main.py --port 5023  (工作目录: py_instances/ghi789/)
```

## 2. 创建实例

### 2.1 通过 Web UI 创建

1. 打开 Web UI → "配置管理"页面
2. 点击"添加实例"按钮
3. **第一步 — 规约与名称**：
   - 规约类型选择"Python微电网"
   - 填写实例名称（如"微电网仿真-1号站"）
4. **第二步 — 网络配置**：
   - Modbus 端口：填写该实例的 Modbus TCP 监听端口（默认 5021，多实例需填不同端口）
   - 轮询间隔：Go 端从模拟器读取数据的周期（默认 1000ms，推荐 500~2000ms）
   - 仿真起始时间：HH:MM 格式（如 08:00），PV/Load 曲线从此时刻开始演算，留空使用当前系统时间
5. **第三步 — 点表与接口**：
   - Python 微电网无需上传点表文件（自动从 device.json 生成）
   - 可选启用 HTTP 接口（用于独立 REST API 访问）
6. **第四步 — 确认**：
   - 检查 Modbus 端口、轮询间隔、仿真起始时间
   - Python 微电网不支持"创建后立即启动"（需先配置设备）

> 创建第二个实例时，Modbus 端口必须与已有实例不同，否则会提示"端口冲突"。

### 2.2 通过 REST API 创建

```bash
curl -X POST http://localhost:8989/api/v1/instances \
  -H "Content-Type: application/json" \
  -d '{
    "name": "微电网站A",
    "protocol": "modbus_bridge",
    "iec104_port": 0,
    "xlsx_file": "",
    "http_enabled": true,
    "http_port": 9001,
    "modbus_bridge_config": {
      "script_dir": "py_simulator",
      "modbus_port": 5021,
      "poll_interval_ms": 1000,
      "start_time": "08:00",
      "device_json": "config/device.json"
    }
  }'
```

## 3. 配置设备

### 3.1 进入配置界面

在配置管理列表中，点击 Python 微电网实例的"Python仿真"按钮 → 进入实例页面 → 切换到"配置管理" tab。

### 3.2 配置文件说明

每个实例使用独立的 `device.json` 文件，位于 `config/py_instances/<id>/config/device.json`（首次启动时从模板复制）。

**JSON 格式示例**：
```json
{
  "start_time": "10:00",
  "Devices": [
    { "Meter": [{ "DeviceKey": "Meter_01", "slave_id": 1 }] },
    { "PV": [{ "DeviceKey": "PV_01", "ratedPower": 100, "mode": 1, "slave_id": 2 }] },
    { "BESS": [{ "DeviceKey": "BESS_01", "ratedCapacity": 200, "slave_id": 3 }] },
    { "EV": [{ "DeviceKey": "EV_01", "PUB_CONN.RatedPW": 22, "slave_id": 4 }] },
    { "Load": [{ "DeviceKey": "LOAD_01", "base_power": 120, "mode": 0, "slave_id": 5 }] }
  ]
}
```

Web UI 配置管理界面直接读写此文件，两者完全等价。

### 3.3 全局设置

| 参数 | 说明 |
|------|------|
| 仿真起始时间 | HH:MM 格式，PV/Load 曲线从此时刻开始演算 |

> 注意：实例级别的 `start_time` 配置优先级高于 device.json 中的值。修改仿真时间需在"编辑实例"中同步修改。

### 3.4 设备类型详细参数

#### PV 光伏

| 参数 | JSON Key | 说明 | 默认值 |
|------|----------|------|--------|
| 设备标识 | DeviceKey | 唯一名称 | 必填 |
| 额定功率 (kW) | ratedPower | 最大出力 | **必填** |
| 出力模式 | mode | 0=CSV 曲线回放, 1=正弦曲线 | 1 |
| CSV 文件 | csv_file | mode=0 时的曲线文件名 | — |
| Slave ID | slave_id | Modbus 从站地址 1~247 | 自动分配 |

**正弦模式**：6:00~18:00 按 sin²(x) 曲线出力，叠加 ±5% 随机波动，夜间为 0。

**CSV 模式**：读取 `config/` 下的 CSV 文件，格式为 `小时,功率(kW)`，每 15 分钟一个点（0.25 步长），中间线性插值。

#### BESS 储能

| 参数 | JSON Key | 说明 | 默认值 |
|------|----------|------|--------|
| 设备标识 | DeviceKey | 唯一名称 | 必填 |
| 额定容量 (kWh) | ratedCapacity | 电池容量 | 200 |
| 最大充电功率 (kW) | maxChargePower | — | 100 |
| 最大放电功率 (kW) | maxDischargePower | — | 100 |
| SOC 上限 (%) | socMax | 0~100 | 90 |
| SOC 下限 (%) | socMin | 0~100 | 10 |
| 初始 SOC | initial_soc | 0~1（0.5 表示 50%） | 0.5 |
| 标称电压 (V) | voltage_nominal | — | 48 |
| 内阻 (Ω) | resistance | — | 0.01 |
| Slave ID | slave_id | — | 自动分配 |

**运行逻辑**：通过 Modbus 写入 `BS.SysAPSetPoint`（寄存器地址 14）下发功率指令，正值放电、负值充电。SOC 实时计算，到限值自动截止。

#### EV 充电桩

| 参数 | JSON Key | 说明 | 默认值 |
|------|----------|------|--------|
| 设备标识 | DeviceKey | 唯一名称 | 必填 |
| 额定功率 (kW) | PUB_CONN.RatedPW | — | **必填** |
| 最小功率 (kW) | PUB_CONN.MinChargePW | — | 0 |
| 充电系数 | charge_factor | 0~1 | 1.0 |
| 充电器类型 | charger_type | "AC" 或 "DC" | AC |
| 相模式 | phase_mode | 1=单相, 3=三相 | 3 |
| 电压 (V) | voltage | AC:200~250, DC:300~500 | AC:220 |
| 功率因数 | power_factor | 0~1 | AC:0.95 |
| 充电时段 | charge_schedule | 数组 [{start, stop}] | [{8:00-10:00}] |
| Slave ID | slave_id | — | 自动分配 |

**充电时段格式**：`[{"start":"08:00","stop":"10:00"}, {"start":"18:00","stop":"20:00"}]`，支持跨零点。仅在时段内充电，时段外功率为 0。

#### Load 负荷

| 参数 | JSON Key | 说明 | 默认值 |
|------|----------|------|--------|
| 设备标识 | DeviceKey | 唯一名称 | 必填 |
| 基础功率 (kW) | base_power | mode=1 时的基准 | 120 |
| 出力模式 | mode | 0=CSV, 1=随机波动(±20%) | 0 |
| CSV 文件 | csv_file | mode=0 时使用 | — |
| Slave ID | slave_id | — | 自动分配 |

#### Meter 电表

| 参数 | JSON Key | 说明 | 默认值 |
|------|----------|------|--------|
| 设备标识 | DeviceKey | 唯一名称 | 必填 |
| Slave ID | slave_id | — | 自动分配 |

电表无需额外配置，自动汇总：`并网功率 = 负荷 + 充电桩 - 光伏 - 储能放电 + 储能充电`

### 3.5 保存与生效

编辑完成后点击"保存配置"按钮。保存后：
- 配置写入实例的 `config/device.json`
- 需要**重启实例**才能使新配置生效
- 运行中的实例不允许编辑配置

**修改配置流程**：停止实例 → 修改配置 → 启动实例

## 4. 启动与运行

### 4.1 启动实例

1. 确保至少配置了 1 个设备
2. 在配置管理列表中点击"启动"
3. 系统自动执行：
   - 创建/复用实例隔离工作目录（`config/py_instances/<id>/`）
   - 启动 Python 进程：`python3 main.py --port <配置的端口>`
   - 等待 Modbus TCP 端口就绪（最长 10 秒）
   - Go 开始以配置的间隔轮询 Modbus 数据
4. 状态变为"运行中"

### 4.2 运行时数据查看

点击配置列表的"Python仿真"按钮进入实例页面：

**拓扑组态 Tab**：
- 显示 SVG 组态图：电网 → 母线 → 各设备
- 实时刷新功率值和 SOC

**Dashboard 数据**：
- 并网总功率（正=用电，负=反送）
- 各 PV 实时功率
- 各 BESS 功率 + SOC + 充放模式
- 充电桩功率
- 负荷功率

### 4.3 设备控制

通过 API 或 Web 界面对运行中的设备下发指令：

| 设备 | 控制项 | IOA 计算 | 说明 |
|------|--------|---------|------|
| PV | 限功率 | 2000 + (序号-1)×100 + 2 | 限制最大出力 (kW) |
| BESS | 功率设定 | 3000 + (序号-1)×100 + 7 | 正值放电、负值充电 (kW) |
| EV | 充电功率设定 | 4000 + (序号-1)×100 + 4 | 设置最大充电功率 (kW) |

**REST API 控制示例**：
```bash
# 设置 BESS_01 放电 50kW（IOA=3007）
curl -X POST http://localhost:8989/api/v1/py-microgrid/<id>/control \
  -H "Content-Type: application/json" \
  -d '{"ioa": 3007, "value": 50}'

# 设置 PV_01 限功率 80kW（IOA=2002）
curl -X POST http://localhost:8989/api/v1/py-microgrid/<id>/control \
  -H "Content-Type: application/json" \
  -d '{"ioa": 2002, "value": 80}'
```

### 4.4 编辑实例配置

在实例停止状态下，点击"编辑"可修改：
- 实例名称
- Modbus 端口（需确保不与其他实例冲突）
- 轮询间隔
- 仿真起始时间
- HTTP 接口开关和端口

## 5. 导出点表

运行中的实例可以导出 Modbus TCP 点表为 Excel 文件：

1. 进入 Python 仿真页面
2. 点击"导出点表"按钮
3. 下载 xlsx 文件

### 5.1 点表 Excel 格式

**Sheet "说明"**：协议类型、数据编码说明、设备列表概览

**Sheet "point"**：

| 列 | 字段 | 说明 |
|----|------|------|
| A | point-name | 测点名（设备名.测点名） |
| B | point-number | IOA 序号 |
| C | value-type | float |
| D | point-type | AI / AO / DI |
| E | coefficient | 系数（1） |
| F | base-value | 基值（0） |
| G | alias | 测点描述 |
| H | register-address | Modbus 寄存器起始地址 |
| I | function-code | 3（读）/ 16（写） |
| J | value-type | SW_FLOAT |

### 5.2 Modbus 通信参数

| 参数 | 值 |
|------|-----|
| 协议 | Modbus TCP |
| 端口 | 实例配置的 Modbus 端口 |
| 数据编码 | 32-bit signed int（实际值 × 100） |
| 字节序 | Big-Endian (ABCD) |
| 寄存器宽度 | 每测点 2 个 16-bit 寄存器 |
| 读取功能码 | FC03 Read Holding Registers |
| 写入功能码 | FC16 Write Multiple Registers |

### 5.3 IOA 分配规则

| 设备类型 | IOA 基址 |
|---------|---------|
| Meter | 1000 + (设备序号-1) × 100 |
| PV | 2000 + (设备序号-1) × 100 |
| BESS | 3000 + (设备序号-1) × 100 |
| EV | 4000 + (设备序号-1) × 100 |
| Load | 5000 + (设备序号-1) × 100 |

## 6. 功率曲线管理

### 6.1 CSV 格式

```csv
小时,功率(kW)
0.0,0.0
0.25,10.0
0.5,20.0
...
24.0,0.0
```

- 第一行为表头（自动跳过）
- 步长 0.25 小时（15 分钟）
- 中间值线性插值

### 6.2 上传曲线

在 Python 仿真页面的"配置管理" tab 底部"功率曲线管理"区域，点击"上传 CSV 曲线"按钮上传文件。

### 6.3 内置曲线

| 文件 | 说明 |
|------|------|
| pv_curve.csv | PV 功率曲线（6:00~18:00 发电，峰值约额定功率） |
| load_curve.csv | 负荷曲线（凌晨低谷 20~30kW，白天高峰 100~120kW） |

## 7. 多实例使用示例

### 7.1 创建两个不同配置的站

```bash
# 站A：端口5021，当前时间开始
curl -X POST http://localhost:8989/api/v1/instances \
  -d '{"name":"站A","protocol":"modbus_bridge","iec104_port":0,"xlsx_file":"",
       "http_enabled":true,"http_port":9001,
       "modbus_bridge_config":{"modbus_port":5021,"poll_interval_ms":1000,"start_time":""}}'

# 站B：端口5022，从12:00开始仿真
curl -X POST http://localhost:8989/api/v1/instances \
  -d '{"name":"站B","protocol":"modbus_bridge","iec104_port":0,"xlsx_file":"",
       "http_enabled":true,"http_port":9002,
       "modbus_bridge_config":{"modbus_port":5022,"poll_interval_ms":1000,"start_time":"12:00"}}'
```

### 7.2 验证独立性

两个实例：
- 各自独立的 Python 进程（`python3 main.py --port 5021` / `--port 5022`）
- 各自独立的 device.json 配置
- 各自独立的日志文件
- 对一个实例的控制操作不会影响另一个

## 8. 从手动 Python 配置迁移

如果你之前独立运行 Python 模拟器（`python3 main.py`），迁移步骤：

1. 将你的 `device.json` 复制到 `config/py_simulator/config/device.json`
2. 将你的 CSV 曲线文件复制到同目录
3. 在 GridSim Web UI 创建 Python 微电网实例
4. 启动实例（系统会从模板复制配置到实例隔离目录）
5. 进入"配置管理" tab 验证配置是否正确加载

配置文件格式完全兼容，无需修改。

## 9. 故障排查

### 9.1 查看实例日志

```bash
# 查看仿真状态日志
cat config/py_instances/<instance-id>/log/simulation.log

# 查看 Modbus 通信日志
cat config/py_instances/<instance-id>/log/message.log

# 查看主程序日志（Go 侧）
cat logs/output.log | grep -i bridge
```

### 9.2 常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| 启动超时 "timeout waiting for port" | Python 进程启动失败或崩溃 | 检查实例日志中的错误信息 |
| 端口冲突 | 另一个实例使用了相同端口 | 修改实例的 Modbus 端口 |
| device.json 解析失败 | JSON 格式错误 | 检查 JSON 语法（括号匹配、逗号等） |
| PV 功率一直为 0 | 仿真时间在夜间（18:00~6:00） | 修改仿真起始时间到白天时段 |
| BESS 功率不响应 | IOA 地址不正确 | 使用导出的点表确认正确的 IOA |
