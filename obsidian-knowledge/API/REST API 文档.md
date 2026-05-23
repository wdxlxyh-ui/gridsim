---
title: REST API 文档
tags:
  - api
  - rest
  - http
aliases:
  - HTTP API Reference
created: 2026-05-23
---

# REST API 文档

Base URL: `http://localhost:8989`

---

## 实例管理 API（Server Mode）`/api/v1`

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/v1/instances` | 列出所有实例 |
| `POST` | `/api/v1/instances` | 创建实例 |
| `GET` | `/api/v1/instances/{id}` | 获取实例详情 |
| `PUT` | `/api/v1/instances/{id}` | 更新实例（部分更新） |
| `DELETE` | `/api/v1/instances/{id}` | 删除实例 |
| `POST` | `/api/v1/instances/{id}/start` | 启动实例 |
| `POST` | `/api/v1/instances/{id}/stop` | 停止实例 |
| `POST` | `/api/v1/instances/{id}/restart` | 重启实例 |
| `GET` | `/api/v1/status` | 全局服务状态 |
| `POST` | `/api/v1/upload` | 上传 .xlsx 点表 |
| `GET` | `/api/v1/files` | 获取已上传文件列表 |
| `GET` | `/api/v1/csv-files` | 获取 CSV 文件列表 |

## 详情页 API（v2.1+）

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `.../points` | 获取所有测点实时快照 |
| `GET` | `.../points/batch?ioas=X,Y,Z` | 批量读取指定 IOA（v2.5.3） |
| `GET` | `.../points/{ioa}` | 获取单个测点 |
| `PUT` | `.../points/{ioa}` | 置数 |
| `GET` | `.../points/auto-change/{ioa}` | 获取自动变化配置 |
| `PUT` | `.../points/auto-change/{ioa}` | 配置自动变化 |
| `DELETE` | `.../points/auto-change/{ioa}` | 删除自动变化配置 |
| `PUT` | `.../points/auto-change/batch` | 批量配置自动变化 |
| `GET` | `.../points/auto-change/export` | 导出自动变化配置 |
| `POST` | `.../points/auto-change/import` | 导入自动变化配置 |
| `GET` | `.../points/export` | 导出测点 CSV 数据 |
| `POST` | `.../{id}/upload-csv` | 上传 CSV 回放文件 |

> 前缀：`/api/v1/instances/{id}`

## 实例级 API（Legacy 模式）`/api`

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/points` | 列出所有测点 |
| `GET` | `/api/points/{ioa}` | 获取单个测点 |
| `PUT` | `/api/points/{ioa}` | 更新测点值 + 触发变化上送 |
| `POST` | `/api/points` | 批量更新测点值 |
| `PUT` | `/api/points/{ioa}/qds` | 更新品质描述 QDS |
| `GET` | `/api/status` | 服务运行状态 |

## 置数示例

```bash
# 遥测
curl -X PUT .../points/16385 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'

# 遥信
curl -X PUT .../points/5 \
  -H 'Content-Type: application/json' \
  -d '{"bool_value": true}'

# 配置自动变化
curl -X PUT .../points/auto-change/16385 \
  -H 'Content-Type: application/json' \
  -d '{"strategy":"increment","enabled":true,"params":{"start_value":0,"step":1,"period_ms":1000,"max_value":100}}'
```

## 错误处理

```json
{
  "error": "错误描述信息"
}
```

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 409 | 冲突 |
| 500 | 服务端内部错误 |

---

相关笔记：[[API/MCP 服务|MCP 服务]]、[[架构设计/数据模型|数据模型]]
