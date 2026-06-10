# 微电网 API v2.5

> Base: `/api/v1/microgrid/{instanceId}`

---

## 1. 拓扑管理

### GET /topology
获取拓扑配置。

```json
{
  "bus_name": "10kV 母线",
  "bus_voltage_kv": 10,
  "grid_meter": { "rated_capacity_kw": 500, "island_mode": false },
  "devices": [{ "id": "dev-1", "type": "pv", "name": "PV-1", ... }],
  "formulas": [{ "id": "auto-grid", "target": "GRID_P", "expression": "..." }]
}
```

### PUT /topology
保存拓扑配置。Body 同 GET 响应格式。

---

## 2. 设备管理

### POST /device — 添加设备
```json
{
  "type": "pv",
  "name": "PV-1",
  "params": { "rated_power_kw": 80 },
  "custom_points": [{ "name": "自定义1", "type": "AI", "alias": "CUSTOM.Val" }]
}
```

### PUT /device — 更新设备
Body 同 POST，需包含 `id`。

### DELETE /device/{devId} — 删除设备

---

## 3. 开关控制

### POST /control/{devId}?closed=true|false

---

## 4. 仪表盘

### GET /dashboard

```json
{
  "grid_power_kw": -35.2,
  "pv": [{ "id": "pv1", "name": "PV-1", "power_kw": 48.5, "closed": true }],
  "battery": [{ "id": "bat1", "name": "ESS-1", "power_kw": -30, "soc": 68 }],
  "load": [{ "id": "load1", "name": "L1", "power_kw": 32 }],
  "charger": [...],
  "total_pv_kw": 48.5, "total_bat_kw": -30,
  "total_load_kw": 32, "total_charger_kw": 0,
  "battery_soc": 68
}
```

> 功率约定: PV≥0(发电), Bat>0=充电/<0=放电, Grid>0=用电/<0=送电

---

## 5. 测点管理

### GET /points

```json
{
  "points": [
    {
      "ioa": 1, "name": "关口表_有功功率", "point_type": "AI",
      "value": -35.2, "unit": "kW",
      "can_toggle": false,    // 是否可切换本地/远方
      "local_mode": false     // 当前是否本地模式
    }
  ]
}
```

---

## 6. 公式管理

### GET /formulas
### POST /formulas — 添加公式
```json
{ "name": "f1", "target": "GRID_P", "expression": "{储能1_充放电功率}+{负荷1_有功功率}", "enabled": true }
```
### PUT /formulas — 更新公式
### DELETE /formulas/{formulaId} — 删除公式

> 自动公式: 引擎启动时自动创建 GRID 公式，ID=`auto-grid`

---

## 7. 导出点表

### GET /export-xlsx

返回 `{instanceId}_points.zip`，内按设备拆分 xlsx：

```
关口表.xlsx    — METER.ActivePW 等
光伏1.xlsx     — INV.GenActivePW 等
储能1.xlsx     — BS.ActivePW + 自定义测点
...
```

每 xlsx 格式：

| point-name | point-number | value-type | point-type | efficient | base-value | alias |
|-----------|-------------|-----------|-----------|---------|----------|-----|
| 关口表_有功功率 | 1 | DOUBLE | AI | 1 | 0 | METER.ActivePW |

> 自定义测点 alias = 用户输入值或测点名

---

## 8. 控制模式

测点 `can_toggle=true` 时，前端显示 本地/远方 开关：
- **远方**: 引擎控制（默认）
- **本地**: 通过 `PUT /api/v1/instances/{id}/points/auto-change/{ioa}` 配置 11 种策略
