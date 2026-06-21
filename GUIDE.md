# GridSim v3.1.0 部署使用指南

> GridSim — IEC104/Modbus 电力仿模拟平台  
> 部署后 AI 助手 / 开发者通过本文件快速了解全部操作方式。

---

## 1. 快速启动

```bash
# Linux
cd bin && ./start.sh          # 启动服务 (默认 :8989)
./stop.sh                     # 停止
./restart.sh                  # 重启

# 手动启动
./gridsim serve --http :8989 --config-dir ../config --log-dir ../logs

# Windows
bin\gridsim.exe serve --http :8989 --config-dir config --log-dir logs
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--http` / `-H` | `:8989` | 管理 API 端口 |
| `--config-dir` / `-c` | `./config` | 配置目录 (instances.json, users.json) |
| `--log-dir` / `-L` | `./logs` | 日志目录 |

**启动后访问：**
- Web UI: `http://<host>:8989`
- API: `http://<host>:8989/api/v1/...`
- OpenAPI Spec: `http://<host>:8989/openapi.json`
- MCP Server: `./bin/gridsim-mcp -simulator http://<host>:8989 -mode both`

**默认账号：** `admin` / `admin123`

---

## 2. REST API 完整接口

### 2.1 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 登录，返回 JWT Token |

登录后在后续请求头中携带：`Authorization: Bearer <token>`

### 2.2 实例管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/instances` | 列出所有实例 |
| POST | `/api/v1/instances` | 创建实例 |
| GET | `/api/v1/instances/{id}` | 获取实例详情 |
| PUT | `/api/v1/instances/{id}` | 更新实例配置 |
| DELETE | `/api/v1/instances/{id}` | 删除实例 |
| POST | `/api/v1/instances/{id}/start` | 启动实例 |
| POST | `/api/v1/instances/{id}/stop` | 停止实例 |
| POST | `/api/v1/instances/{id}/restart` | 重启实例 |

**创建实例示例：**
```bash
curl -X POST http://localhost:8989/api/v1/instances \
  -H "Content-Type: application/json" \
  -d '{"name":"变电站A","iec104_port":2404,"xlsx_file":"point.xlsx"}'
```

**创建 Modbus 实例：**
```json
{
  "name": "Modbus设备",
  "iec104_port": 2502,
  "xlsx_file": "modbus_points.xlsx",
  "protocol": "modbus_tcp",
  "modbus_config": {"port": 2502, "slave_id": 1}
}
```

### 2.3 测点读写

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/instances/{id}/points` | 获取所有测点快照 |
| GET | `/api/v1/instances/{id}/points/batch?ioas=1,2,3` | 批量读取指定 IOA |
| GET | `/api/v1/instances/{id}/points/{ioa}` | 获取单个测点 |
| PUT | `/api/v1/instances/{id}/points/{ioa}` | **置数**（触发变化上送 COT=3） |
| POST | `/api/v1/instances/{id}/points/batch` | 批量置数 |
| GET | `/api/v1/instances/{id}/points/export` | 导出 CSV |

**置数 — 按测点类型使用不同字段：**
```bash
# AI 遥测
curl -X PUT .../points/16385 -d '{"value": 235.5}'

# DI 遥信
curl -X PUT .../points/5 -d '{"bool_value": true}'

# PI 遥脉
curl -X PUT .../points/10 -d '{"int_value": 42}'
```

### 2.4 自动变化策略

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `.../points/auto-change/{ioa}` | 查看策略 |
| PUT | `.../points/auto-change/{ioa}` | 配置策略 |
| DELETE | `.../points/auto-change/{ioa}` | 删除策略 |
| PUT | `.../points/auto-change/batch` | 批量配置 |
| GET | `.../points/auto-change/export` | 导出 CSV |
| POST | `.../points/auto-change/import` | 导入 CSV |

**11 种策略：**

| 策略 | 说明 | 关键参数 |
|------|------|----------|
| `increment` | 递增 | `start_value, step, period_ms, max_value` |
| `random` | 随机 | `min_value, max_value_r, period_ms, decimal_places` |
| `csv` | CSV 回放 | `csv_file, time_format, csv_column_map` |
| `max` | 取大值 | `para_a: "ioa1;ioa2"` |
| `min` | 取小值 | 同 max |
| `soc` | SOC 计算 | `init_soc, rated_cap, power_ioa, integral_ms` |
| `energy` | 电量统计 | `init_energy, stat_type(0充/1放), energy_power_ioa` |
| `aofollow` | AO 关联 | `follow_ao_ioa` |
| `apiupdate` | 接口更新 | `api_init_value` (仅允许 API 写入) |
| `manual` | 手动 | 无参数 (需 API 置数) |
| `custom` | 自定义公式 | `custom_ioas, custom_formula, period_ms` |

### 2.5 CSV 回放

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `.../upload-csv` | 上传 CSV 文件 |
| GET | `.../csv-files` | 列出 CSV 文件 |
| GET | `.../csv-content/{filename}` | 预览 CSV 内容 |
| POST | `.../csv-replay` | 配置多测点同步回放 |
| POST | `.../batch-replay` | 批量顺序回放 |
| GET | `.../batch-replay/{batch_id}` | 查询回放进度 |

### 2.6 微电网

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/PUT | `/api/v1/microgrid/{id}/topology` | 获取/保存拓扑 |
| POST/PUT/DELETE | `/api/v1/microgrid/{id}/device` | 设备增删改 |
| POST | `/api/v1/microgrid/{id}/control/{devId}?closed=true` | 开关控制 |
| GET | `/api/v1/microgrid/{id}/dashboard` | 仪表盘数据 |
| GET | `/api/v1/microgrid/{id}/points` | 测点列表 |
| CRUD | `/api/v1/microgrid/{id}/formulas` | 公式管理 |
| GET | `/api/v1/microgrid/{id}/export-xlsx` | 导出点表 |

### 2.7 接口测试 (Proxy)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/proxy` | 执行 HTTP 代理请求 |
| GET/POST | `/api/v1/proxy/collections` | 接口集合管理 |
| DELETE | `/api/v1/proxy/collections/{id}` | 删除接口 |
| GET/POST | `/api/v1/proxy/environments` | 环境变量管理 |
| POST | `/api/v1/proxy/environments/{id}/activate` | 激活环境 |
| GET | `/api/v1/proxy/export` | 导出全部配置 |

### 2.8 AI 增强接口 (v3.1+)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/openapi.json` | OpenAPI 3.0 规范 (30+ 端点) |
| GET | `/api/v1/state` | **统一状态** — 单次调用获取全部实例+状态 |
| GET | `/api/v1/events` | **SSE 事件流** — 实时推送变更 |
| GET/POST | `/api/v1/recordings` | **场景录制** — start/stop/list |

**幂等性：** POST/PUT/DELETE 请求携带 `Idempotency-Key: <唯一键>` 头，重复请求返回缓存结果（24h 有效）。

### 2.9 全局接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/status` | 全局状态 (版本/实例计数) |
| GET | `/api/v1/files` | 列出点表文件 |
| POST | `/api/v1/upload` | 上传点表文件 (.xlsx/.csv) |
| GET | `/api/v1/protocols` | 支持的协议列表 |

---

## 3. 错误格式

所有错误响应使用统一结构化格式：

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "instance abc123 not found",
    "hint": "Use GET /api/v1/instances to list available instances",
    "candidates": ["substation-a", "substation-b"],
    "field": "id"
  }
}
```

| code | HTTP | 含义 |
|------|------|------|
| `BAD_REQUEST` | 400 | 参数错误 |
| `UNAUTHORIZED` | 401 | 认证失败 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 冲突 (端口占用/已存在) |
| `METHOD_NOT_ALLOWED` | 405 | 方法不允许 |
| `INTERNAL_ERROR` | 500 | 内部错误 |
| `INSTANCE_NOT_FOUND` | 404 | 实例不存在 |
| `INSTANCE_NOT_RUNNING` | 400 | 实例未运行 |
| `PORT_IN_USE` | 409 | 端口已占用 |
| `INVALID_JSON` | 400 | JSON 格式错误 |

---

## 4. MCP 工具完整列表

MCP (Model Context Protocol) 服务器让 AI 助手直接控制模拟器。

### 启动 MCP 服务器

```bash
./bin/gridsim-mcp -simulator http://localhost:8989 -mode both
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-simulator` | `http://localhost:8989` | GridSim 地址 |
| `-mode` | `both` | `instance`(实例管理) / `data`(数据接口) / `both`(全部) |

### Claude Desktop 配置

```json
{
  "mcpServers": {
    "gridsim": {
      "command": "/path/to/gridsim-mcp",
      "args": ["-simulator", "http://localhost:8989", "-mode", "both"]
    }
  }
}
```

### 实例管理工具 (11 个)

| 工具 | 参数 | 说明 |
|------|------|------|
| `list_instances` | — | 列出所有实例 |
| `get_instance` | `instance_id` | 获取实例详情 |
| `create_instance` | `config`(JSON) | 创建实例 |
| `update_instance` | `instance_id, config` | 更新实例 |
| `delete_instance` | `instance_id` | 删除实例 |
| `start_instance` | `instance_id` | 启动实例 |
| `stop_instance` | `instance_id` | 停止实例 |
| `restart_instance` | `instance_id` | 重启实例 |
| `get_server_status` | — | 全局状态 |
| `get_state` | — | **统一状态** (单次调用) |
| `get_openapi_spec` | — | **API 规范** (自动发现) |

### 测点数据工具 (15 个)

| 工具 | 参数 | 说明 |
|------|------|------|
| `list_points` | `instance_id` | 列出所有测点 |
| `read_point` | `instance_id, ioa` | 读取单个测点 |
| `read_points` | `instance_id, ioas?` | 批量读取 |
| `write_point` | `instance_id, ioa, value/bool_value/int_value` | **置数** |
| `write_points` | `instance_id, points[]` | **批量置数** |
| `config_auto_change` | `instance_id, ioa, strategy, enabled, params` | **配置策略** |
| `get_auto_change` | `instance_id, ioa` | 查看策略 |
| `delete_auto_change` | `instance_id, ioa` | 删除策略 |
| `batch_config_auto_change` | `instance_id, ioas[], strategy, enabled, params` | 批量配置 |
| `export_auto_changes` | `instance_id` | 导出 CSV |
| `import_auto_changes` | `instance_id, csv_content` | 导入 CSV |
| `upload_csv` | `instance_id, csv_content` | 上传 CSV |
| `list_csv_files` | `instance_id` | 列出 CSV |
| `config_csv_replay` | `instance_id, csv_file, mappings[]` | **CSV 回放** |
| `update_qds` | `ioa, invalid/not_topical/...` | 更新品质描述 |

### 接口测试工具 (9 个)

| 工具 | 参数 | 说明 |
|------|------|------|
| `proxy_request` | `method, url, headers?, body?, timeout?` | **发送 HTTP 请求** |
| `get_collections` | — | 获取接口集合 |
| `save_collection` | `item`(JSON) | 保存接口/文件夹 |
| `delete_collection` | `id` | 删除接口 |
| `get_environments` | — | 获取环境列表 |
| `save_environment` | `env`(JSON) | 保存环境 |
| `delete_environment` | `id` | 删除环境 |
| `activate_environment` | `id` | 激活环境 |
| `export_proxy_config` | — | 导出全部配置 |

### 全局工具 (2 个)

| 工具 | 参数 | 说明 |
|------|------|------|
| `list_files` | — | 列出点表文件 |
| `get_protocols` | — | 支持的协议 |

**工具总计：37 个** (实例管理 11 + 数据接口 15 + 接口测试 9 + 全局 2)

---

## 5. Web GUI 页面

浏览器访问 `http://<host>:8989` 打开 Web 管理界面。

| 页面 | 路由 | 功能 |
|------|------|------|
| **仪表盘** | `/dashboard` | 实例运行总览、统计卡片、规约分布、快捷操作（v3.1） |
| **配置管理** | `/config` | 实例 CRUD、上传点表、启停控制、实例容量概览 |
| **运行监控** | `/monitor` | 全局实例状态看板（自动刷新） |
| **实例详情** | `/detail/:id` | 测点实时刷新(100ms~5s)、置数、自动变化配置、CSV 回放 |
| **趋势对比** | `/trend` | ECharts 多线趋势图，支持缩放/拖拽/十字准线 |
| **接口测试** | `/proxy` | Postman 风格 HTTP 测试工具，集合管理、环境变量、前后置脚本 |
| **微电网** | `/microgrid` | 拓扑编辑器(PV/Battery/Load/Charger)、SVG 实时渲染、功率流动画 |

### GUI 核心操作

1. **操作引导** — 点击右下角 ❓ 按钮，选择"基础引导"或"高级引导"，跟随步骤熟悉全功能（v3.1）
2. **创建实例** — 配置管理页 → "新建实例" → 填写名称、端口、选择点表文件
3. **启动实例** — 配置管理页 → 点击实例的 "启动" 按钮
4. **置数** — 实例详情页 → 找到测点 → 输入值 → 点击 "置数"
5. **配置自动变化** — 实例详情页 → 策略 Tab → 选择策略 → 设置参数
6. **CSV 回放** — 实例详情页 → CSV 回放卡片 → 上传文件 → 配置列映射
7. **趋势查看** — 趋势对比页 → 选择实例和测点 → 实时观察曲线

---

## 6. 目录结构

```
gridsim/
├── bin/
│   ├── gridsim              # 主程序
│   ├── gridsim-mcp          # MCP 服务器
│   ├── VERSION              # 版本号
│   ├── start.sh             # 启动脚本 (Linux)
│   ├── stop.sh              # 停止脚本 (Linux)
│   └── restart.sh           # 重启脚本 (Linux)
├── config/
│   ├── instances.json       # 实例配置
│   ├── users.json           # 用户认证配置
│   ├── auto_changes/        # 自动变化策略持久化
│   ├── csv/                 # CSV 回放文件
│   └── recordings/          # 场景录制文件
├── logs/
├── resources/               # 内置资源 (协议模板等)
└── web/dist/                # 前端静态文件
```

---

## 7. 典型工作流

### 流程 1: 创建仿真并启动

```bash
# 1. 上传点表
curl -X POST http://localhost:8989/api/v1/upload -F "file=@point.xlsx"

# 2. 创建实例
curl -X POST http://localhost:8989/api/v1/instances \
  -H "Content-Type: application/json" \
  -d '{"name":"变电站A","iec104_port":2404,"xlsx_file":"point.xlsx"}'

# 3. 启动实例 (使用返回的 id)
curl -X POST http://localhost:8989/api/v1/instances/{id}/start

# 4. 查看状态
curl http://localhost:8989/api/v1/state
```

### 流程 2: 模拟数据变化

```bash
# 方式 A: 直接置数
curl -X PUT http://localhost:8989/api/v1/instances/{id}/points/16385 \
  -d '{"value": 235.5}'

# 方式 B: 配置自动递增
curl -X PUT http://localhost:8989/api/v1/instances/{id}/points/auto-change/16385 \
  -d '{"strategy":"increment","enabled":true,"params":{"start_value":0,"step":1,"period_ms":1000,"max_value":100}}'
```

### 流程 3: MCP 控制 (Python 伪代码)

```python
# 查看状态
get_state()

# 创建并启动
create_instance(config='{"name":"Test","iec104_port":2404,"xlsx_file":"point.xlsx"}')
start_instance(instance_id="xxx")

# 置数
write_point(instance_id="xxx", ioa=16385, value=220.5)

# 配置自动变化
config_auto_change(instance_id="xxx", ioa=16385, strategy="random",
    enabled=True, params='{"min_value":200,"max_value_r":240,"period_ms":500}')
```

---

## 8. 协议支持

| 协议 | 端口 | 说明 |
|------|------|------|
| IEC 104 | 2404 (默认) | 遥测 AI / 遥信 DI / 遥脉 PI / 遥控 DO / 遥调 AO |
| Modbus TCP | 2502 (示例) | 功能码 1/3/4/5/6/16 |
| Microgrid | 2404 | 微电网仿真 (PV/Battery/Load/Charger) |

### IEC 104 测点类型

| 类型 | 功能 | 类型标识 | 数据 |
|------|------|----------|------|
| AI | 遥测 (模拟量) | M_ME_NC_1 (13) | float32 |
| DI | 遥信 (数字量) | M_SP_NA_1 (1) | bool |
| PI | 遥脉 (累计量) | M_IT_NA_1 (15) | int32 |
| DO | 遥控 | C_SC_NA_1 (45) | 接收外部控制 |
| AO | 遥调 | C_SE_NC_1 (48) | 接收外部控制 |

---

## 9. 版本

查看版本：`cat bin/VERSION` 或 `curl http://localhost:8989/api/v1/status`
