# GridSim 用户手册

## 1. 介绍

GridSim 是一款多协议电力系统仿真工具，支持 IEC 60870-5-104、Modbus TCP、微电网仿真等多种协议的模拟器功能。主要用于：

- 电力 SCADA/EMS 系统的联调测试
- Modbus TCP 设备仿真
- 微电网场景（光伏、储能、充电桩、负荷）的物理模型仿真
- IEC104 主站/从站模式测试

### 1.1 支持的协议类型

| 协议 | 说明 |
|------|------|
| IEC104 | 标准 IEC 60870-5-104 从站模式，响应总召、变化上送、AO/DO 控制 |
| IEC104 客户端 | 主站模式，连接远端从站，接收遥测/遥信，发送遥控/遥调 |
| Modbus TCP | Modbus TCP 从站，支持 FC01~06、0F、10 功能码 |
| 微电网 | Go 原生微电网物理模型仿真（拓扑编辑器） |
| Python 微电网 | 基于 Python 物理模型的微电网仿真（PV/BESS/EV/Load/Meter），支持多实例 |

### 1.2 系统架构

```
┌──────────────────────────────────────────────────────┐
│               GridSim 主程序 (Go)                     │
│  ┌────────────┐  ┌──────────────────┐                │
│  │ 多实例管理  │  │ 自动变化策略引擎  │                │
│  └────────────┘  └──────────────────┘                │
│  ┌────────────────────────────────────────────────┐  │
│  │ 协议层: IEC104 / Modbus / Bridge / Microgrid   │  │
│  └────────────────────────────────────────────────┘  │
│  ┌────────────┐  ┌──────────────────┐                │
│  │  REST API  │  │  Web UI (Vue 3)  │                │
│  └────────────┘  └──────────────────┘                │
│  ┌────────────────────────────────────────────────┐  │
│  │ Python 微电网进程池 (每实例独立 python3 进程)    │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

## 2. 部署

### 2.1 系统要求

- 操作系统：Linux (amd64/arm64) 或 Windows (amd64)
- 内存：≥ 512MB
- 磁盘：≥ 100MB
- 端口：8989（Web 管理）+ 各实例配置的协议端口
- Python 3.8+（仅 Python 微电网实例需要，如有编译二进制则不需要）

### 2.2 获取安装包

**方式一：自解压安装包（推荐，一键部署）**

```bash
# AMD64 服务器
bash gridsim-install-vX.X.X-linux-amd64.sh

# ARM64 服务器
bash gridsim-install-vX.X.X-linux-arm64.sh

# 指定部署目录
bash gridsim-install-vX.X.X-linux-amd64.sh --target /opt/gridsim
```

自解压安装包会自动完成：停止旧服务 → 备份配置 → 解压部署 → 恢复配置 → 注册 systemd → 启动 → 健康检查。

**方式二：手动解压**

从构建产物中获取对应平台的压缩包：

- `gridsim-vX.X.X-linux-amd64.tar.gz`
- `gridsim-vX.X.X-linux-arm64.tar.gz`
- `gridsim-vX.X.X-windows-amd64.zip`

### 2.3 手动解压部署

**Linux:**
```bash
tar xzf gridsim-vX.X.X-linux-amd64.tar.gz
cd gridsim-vX.X.X-linux-amd64/
./bin/start.sh
```

**Windows:**
解压 zip 文件到任意目录，双击 `bin/gridsim.exe`。

### 2.4 目录结构

```
gridsim/
├── bin/
│   ├── gridsim                 # 主程序
│   ├── gridsim-mcp             # MCP Server（AI Agent 调用）
│   ├── start.sh / start.bat    # 启动脚本
│   ├── stop.sh / stop.bat      # 停止脚本
│   ├── restart.sh              # 重启脚本
│   └── VERSION                 # 版本号
├── config/
│   ├── instances.json          # 实例配置（自动生成）
│   ├── users.json              # 用户认证配置
│   ├── py_simulator/           # Python 微电网模板目录
│   │   ├── main.py             # Python 模拟器入口
│   │   ├── src/                # 设备模型代码
│   │   ├── utils/              # 工具函数
│   │   └── config/             # 默认设备配置
│   │       ├── device.json     # 设备拓扑与参数
│   │       ├── pv_curve.csv    # PV 功率曲线
│   │       ├── load_curve.csv  # 负荷功率曲线
│   │       └── modbus_registers.py  # Modbus 寄存器映射
│   └── py_instances/           # [运行时生成] 每个 Python 实例的隔离工作目录
│       ├── <instance-id-1>/
│       │   ├── main.py         # 从模板复制
│       │   ├── src/            # 从模板复制
│       │   ├── config/         # 独立的设备配置
│       │   └── log/            # 独立的日志目录
│       └── <instance-id-2>/
├── web/dist/                   # 前端静态文件
├── logs/                       # 主程序运行日志
├── backups/                    # 配置备份（部署脚本自动管理）
├── GUIDE.md                    # 快速指南
└── README.md                   # 项目说明
```

### 2.5 使用 deploy-local.sh 部署

如果是在编译服务器本地部署：

```bash
# 自动选择最新版本部署
bash deploy-local.sh

# 指定包部署
bash deploy-local.sh dist/gridsim-v3.2.0-dev-15.120a9ca-linux-amd64.tar.gz

# 查看可用版本
bash deploy-local.sh --list

# 回滚
bash deploy-local.sh --rollback
```

## 3. 启动与停止

### 3.1 Linux 启动

```bash
# 使用启动脚本（推荐）
./bin/start.sh

# 或通过 systemd
systemctl start gridsim
systemctl enable gridsim   # 开机自启
```

直接运行（调试用）：
```bash
./bin/gridsim serve --http :8989 --config-dir ./config --log-dir ./logs --log info
```

### 3.2 Windows 启动

双击 `bin/gridsim.exe` 即可（GUI 模式，自动在系统托盘运行）。

命令行模式：
```cmd
bin\gridsim.exe serve --http :8989 --config-dir config
```

### 3.3 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--http` | `:8989` | Web 管理 API 监听地址 |
| `--config-dir` | `./config` | 配置文件目录 |
| `--log-dir` | `./logs` | 日志目录 |
| `--log` | `info` | 日志级别：debug/info/warn/error |

### 3.4 停止

```bash
./bin/stop.sh
# 或
systemctl stop gridsim
```

### 3.5 访问 Web UI

启动后打开浏览器访问：`http://<服务器IP>:8989`

默认账号：`admin` / `admin123`

## 4. 使用指南

### 4.1 仪表盘

首页展示所有实例的概览卡片：

- 运行状态（运行中/已停止/错误）
- 协议类型标签
- 连接状态（在线/离线）
- 测点数量
- 运行时长
- 端口信息：
  - IEC104/Modbus 实例显示对应端口号
  - Python 微电网实例显示 Modbus 端口号
  - IEC104 客户端显示远端地址

### 4.2 配置管理

通过"配置管理"页面管理所有实例。

#### 创建实例（四步向导）

**第一步 — 规约与名称：**
1. 选择规约类型（IEC104 / IEC104 客户端 / Modbus TCP / 微电网 / Python 微电网）
2. 填写实例名称

**第二步 — 网络配置：**
- IEC104：填写 IEC104 端口（默认 2404）
- IEC104 客户端：填写远端地址、端口、公共地址、重连间隔、总召周期、控制模式
- Modbus TCP：填写 Modbus 端口、从站地址、字节序
- Python 微电网：填写 Modbus 端口（默认 5021）、轮询间隔（默认 1000ms）、仿真起始时间

**第三步 — 点表与接口：**
- IEC104/Modbus：选择或上传 xlsx 点表文件
- 微电网/Python 微电网：无需点表，使用专属编辑器配置
- HTTP 接口开关和端口

**第四步 — 确认：**
- 检查配置摘要
- IEC104/Modbus 可选"创建后立即启动"
- Python 微电网/微电网类型需先配置设备，不支持立即启动

#### 编辑实例

在配置列表中点击"编辑"按钮（实例需先停止）：
- 规约类型不可修改
- IEC104 客户端：可修改远端地址、端口等连接参数
- Python 微电网：可修改 Modbus 端口、轮询间隔、仿真起始时间
- 其他类型：可修改端口号

#### 批量操作

- 勾选多个实例 → 批量启动/停止/删除
- 全选/取消选择

### 4.3 运行监控

"运行监控"页面提供所有实例的实时状态监控卡片：

| 字段 | 说明 |
|------|------|
| Modbus 端口 / IEC104 端口 | 根据协议类型显示对应端口 |
| HTTP 端口 | 如果启用 |
| 客户端状态 | 在线/未连接 |
| 测点数 | 总测点数量 |
| 运行时间 | 自启动以来的时间 |
| 轮询次数/总召次数 | 根据协议类型显示（Python 微电网为轮询次数） |
| 变化上送 | 主动上送的测点变化数 |

操作按钮：
- 运行中实例：详情/Python仿真/微电网 + 重启 + 停止
- 停止实例：启动

### 4.4 实例详情

点击运行中的 IEC104/Modbus 实例进入详情页：
- 实时测点数据表格
- 自动变化策略配置（随机/递增/CSV 回放/自定义公式等）
- CSV 多测点同步回放
- 趋势图

### 4.5 接口测试

内置 API 测试工具（代理页面），支持发送 HTTP 请求测试远端接口。

## 5. 多实例端口管理

### 5.1 端口规划建议

| 用途 | 建议范围 |
|------|---------|
| Web 管理 API | 8989（固定） |
| IEC104 实例 | 2404~2499 |
| Modbus TCP 实例 | 502, 5502~5599 |
| Python 微电网 Modbus 端口 | 5021~5099 |
| 实例 HTTP API | 9001~9099 |

### 5.2 端口冲突检测

系统自动检测以下冲突：
- IEC104/Modbus 端口不能与其他实例重复
- Python 微电网的 Modbus 端口不能与其他 Python 实例重复
- HTTP 端口不能重复
- 创建时发现冲突会报错提示

## 6. 构建与发布

### 6.1 一键发布

```bash
# 完整构建 + 双架构安装包
bash release.sh

# 快速构建（跳过前端类型检查）
bash release.sh --fast

# 仅后端构建
bash release.sh --skip-web
```

产物：
- `gridsim-install-vX.X.X-linux-amd64.sh`（AMD64 自解压安装包）
- `gridsim-install-vX.X.X-linux-arm64.sh`（ARM64 自解压安装包）

### 6.2 单独构建

```bash
# 完整构建
bash build.sh

# 仅后端
bash build.sh --skip-web

# 快速（跳过类型检查）
bash build.sh --fast
```

### 6.3 单独生成安装包

```bash
# 双架构
bash make-installer.sh --all

# 仅 amd64
bash make-installer.sh

# 指定架构
bash make-installer.sh --arch arm64
```

## 7. 常见问题

### Q: Python 微电网实例启动失败，提示"timeout waiting for port"

A: 检查：
1. 目标服务器是否安装了 Python 3.8+（`python3 --version`）
2. 端口是否被其他进程占用（`ss -tlnp | grep 5021`）
3. 查看实例日志：`config/py_instances/<id>/log/simulation.log`

### Q: 修改了 Python 微电网设备配置但没生效

A: 配置修改需要重启实例才能生效。流程：停止实例 → 修改配置 → 启动实例。

### Q: 如何在没有 Python 的服务器上运行 Python 微电网

A: 需要将 `bin/py-microgrid-sim/` 目录（PyInstaller 编译的二进制）放入部署包。如果没有此目录，系统会 fallback 到调用系统 `python3`。

### Q: 两个 Python 微电网实例能否使用相同端口

A: 不能。系统创建时会检测 Modbus 端口冲突，相同端口会被拒绝。
