---
title: 微电网 API
tags:
  - microgrid
  - api
  - rest
aliases:
  - Microgrid REST API
created: 2026-05-23
---

# 微电网 API

> 基础路径: `/api/v1`

## 端点列表

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/v1/microgrid/{id}/topology` | 获取拓扑配置 |
| PUT | `/api/v1/microgrid/{id}/topology` | 更新拓扑配置 |
| POST | `/api/v1/microgrid/{id}/device` | 新增设备 |
| PUT | `/api/v1/microgrid/{id}/device/{devId}` | 更新设备 |
| DELETE | `/api/v1/microgrid/{id}/device/{devId}` | 删除设备 |
| POST | `/api/v1/microgrid/{id}/control/{devId}` | 开关控制 |
| GET | `/api/v1/microgrid/{id}/dashboard` | 实时数据（所有设备功率） |
| GET | `/api/v1/microgrid/{id}/points` | IEC104 测点列表 |
| GET/POST/PUT/DELETE | `/api/v1/microgrid/{id}/formulas` | 自定义公式 CRUD |
| GET | `/api/v1/microgrid/{id}/export-xlsx` | 导出标准点表 xlsx |

## 仪表盘响应

```json
{
  "grid_power_kw": -35.2,
  "pv": [{"id": "pv1", "name": "PV-1", "power_kw": 48.5, "closed": true}],
  "battery": [{"id": "bat1", "name": "ESS-1", "power_kw": -30, "soc": 68, "closed": true}],
  "load": [...],
  "charger": [...],
  "total_pv_kw": 48.5,
  "total_bat_kw": -30,
  "total_load_kw": 32,
  "total_charger_kw": 0,
  "battery_soc": 68
}
```

## 测点响应

```json
{"ioa": 101, "name": "PV-1_Power", "point_type": "AI", "value": 48.5, "unit": "kW", "managed": true}
```

> `managed: true` 表示该测点由引擎管理，不可手动操作。

## 公式语法

`{测点名}` 引用实时值，支持运算符：`+ - * / ( )`

示例: `({PV-1_Power} + {ESS-1_Power}) * 0.9`

引擎自动生成 GRID 公式：
```
GRID_P = (Load_Power + Charger_Power + Bat_Power) - (PV_Power)
```

---

## 相关笔记

- [[微电网/微电网概述|微电网概述]]
- [[微电网/微电网架构与引擎|微电网架构与引擎]]
- [[API/REST API 文档|REST API 文档]]
