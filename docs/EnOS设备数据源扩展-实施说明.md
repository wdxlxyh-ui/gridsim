# EnOS 设备数据源扩展功能实施说明

## 功能概述

本次扩展实现了所有设备类型（PV、BESS、EV、关口表、Load、Wind）的 EnOS 数据源支持，用户可以选择 EnOS 资产和测点标识符进行历史数据回放。

## 主要特性

### 1. 全设备 EnOS 数据源支持
- **关口表 (Meter)**: 支持 `calculated` (功率平衡计算) 和 `enos` (直接回放) 两种模式
- **光伏 (PV)**: 支持 `synthetic`、`csv`、`enos` 三种数据源
- **储能 (BESS)**: 支持功率和 SOC 的独立 EnOS 测点配置
- **充电桩 (EV)**: 支持功率数据的 EnOS 回放
- **负载 (Load)**: 扩展支持 EnOS 数据源（原有 CSV、synthetic 保留）
- **风机 (Wind)**: 保持原有三种模式（CSV、单资产、聚合），增强测点配置

### 2. EnOS 凭据管理
- 独立的凭据档案系统，支持多环境配置
- 凭据安全存储（AES-256-GCM 加密）
- 凭据连接测试功能
- Web 界面统一管理

### 3. 规范配置系统 (Schema v2)
- 统一的配置模型和 API
- ETag 版本控制防止并发修改
- 配置校验和错误提示
- 实时配置生效和状态监控

## 技术架构

### 后端实现
```
/api/v1/py-microgrid/{id}/spec     # 规范配置 GET/PUT
/api/v1/py-microgrid/{id}/validate # 配置校验 POST
/api/v1/py-microgrid/{id}/runtime  # 运行状态 GET
/api/v1/secrets/enos               # 凭据管理 CRUD + 测试
```

### 前端组件
- `EnOSCredentials.vue`: 凭据管理界面
- `DataSourceConfig.vue`: 设备数据源配置组件
- `PyMicrogridSpecConfig.vue`: 规范配置主界面

### 配置文件结构
```
config/
├── enos_credentials/           # EnOS 凭据档案
│   └── enos-prod-cn5.json
└── py_microgrid_instances/     # 实例规范配置
    └── {instance-id}/
        └── spec.json
```

## 使用指南

### 1. 配置 EnOS 凭据
1. 访问凭据管理页面
2. 创建新凭据档案，输入：
   - 配置名称（如"生产环境 CN5"）
   - API 网关地址（必须 HTTPS）
   - Organization ID
   - Access Key / Secret Key
3. 测试连接确保凭据有效

### 2. 配置设备数据源
1. 进入 Python 微电网配置页面
2. 启用 "EnOS 数据回放"
3. 选择凭据档案
4. 对每个设备配置数据源：
   - 选择数据源类型为 "EnOS 回放"
   - 输入资产 ID
   - 配置相应的测点标识符

### 3. 设备特定配置

#### PV 设备
```json
{
  "source": {
    "kind": "enos",
    "enos": {
      "asset_id": "pv_asset_001",
      "power_point": "INV.GenActivePW"
    }
  }
}
```

#### BESS 设备
```json
{
  "source": {
    "kind": "enos",
    "enos": {
      "asset_id": "bess_asset_001", 
      "power_point": "BS.ActivePW",
      "soc_point": "BS.SOC"          // 可选
    }
  }
}
```

#### Wind 聚合设备
```json
{
  "source": {
    "kind": "enos_aggregate",
    "enos": {
      "aggregate_asset_ids": ["wind_001", "wind_002"],
      "theory_power_point": "WNAC.TheoryActivePW",
      "wind_speed_point": "WNAC.WindSpeed",
      "min_coverage_ratio": 0.8
    }
  }
}
```

## 配置参数说明

### TSDB 回放参数
- `fetch_interval_sec`: 拉取间隔（默认 30 秒）
- `lookback_sec`: 查询回溯时长（默认 300 秒）
- `delay_sec`: 查询延迟时间（默认 10 秒）
- `target_lag_sec`: 目标回放滞后（默认 60 秒）
- `max_staleness_sec`: 数据失效阈值（默认 180 秒）
- `failure_policy`: 失效策略（`hold`/`zero`/`stop`）

### 约束条件
- `delay_sec ≤ target_lag_sec ≤ delay_sec + lookback_sec`
- `cache_retention_sec ≥ max(lookback_sec, target_lag_sec + 2×fetch_interval_sec)`
- `max_staleness_sec ≥ fetch_interval_sec`

## 示例配置

参考文件：`config/py_microgrid_instances/example/spec.json`

该示例展示了：
- 关口表使用 EnOS 数据源
- PV、BESS、EV、Load 的 EnOS 配置
- Wind 单资产和聚合两种配置方式
- 完整的回放参数配置

## 迁移指南

### 从旧版本迁移
1. 现有 `device.json` 配置保持兼容
2. 新实例建议使用规范配置 API
3. 旧实例可逐步迁移到新的配置模型

### 配置转换
- 旧的 `mode` 字段映射：
  - `mode=0` → `source.kind=csv`
  - `mode=1` → `source.kind=synthetic`  
  - `mode=2` → `source.kind=enos`
  - `mode=3` → `source.kind=enos_aggregate` (Wind only)

## 注意事项

1. **凭据安全**: 
   - Access Key 和 Secret Key 加密存储
   - 定期轮换凭据
   - 生产环境使用专用凭据档案

2. **网络要求**:
   - API 网关必须使用 HTTPS
   - 确保网络连通性和防火墙配置

3. **数据质量**:
   - 监控数据新鲜度和覆盖率
   - 配置合理的失效策略
   - 定期检查测点标识符的正确性

4. **性能优化**:
   - 根据实际需求调整拉取间隔
   - 控制聚合资产数量
   - 监控缓存使用情况

## 后续开发

1. **Python 运行时适配**: 
   - 修改 Python 模拟器支持新的配置模型
   - 实现 EnOS TSDB 客户端和回放逻辑
   - 增强设备模型支持 EnOS 数据输入

2. **监控和诊断**:
   - 实时状态监控 API 实现
   - 数据质量和连接健康监控
   - 告警和故障自恢复机制

3. **用户体验优化**:
   - 资产和测点的自动发现
   - 配置模板和向导
   - 批量设备配置和导入导出