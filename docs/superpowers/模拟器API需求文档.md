# IEC104 模拟器 API 新增需求

> 提交给模拟器开发团队
> 日期: 2026-05-24
> 模拟器地址: 10.65.99.13:8989

---

## 背景

EGC 特高压功能自动化测试中，需要替代以下低效操作：

| 当前做法 | 问题 | 替代方案 |
|---------|------|---------|
| SSH 到 EGC 下载日志 → Python 解析 count/cmd | 步骤多、慢（~120s/条）、依赖 SSH 权限 | **GET /metrics** 直接返回 |
| 逐条 POST csv-replay + sleep + 读取 | 43条需手动循环 | **batch-replay** 批量提交，回包自带结果 |

---

## 需求 1：GET /metrics（P0）

### 用途

替代 SSH 日志解析，直接返回 DL 实例上 dlPulseSignal 算法的当前状态。

### 接口

```
GET /api/v1/instances/{instance_id}/metrics
```

### 请求

无请求体。

### 响应（成功，200）

```json
{
  "p_edc": 0.0,
  "count": 8,
  "cmd": 80.0,
  "pulse_status": "IDLE_PERIOD"
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `p_edc` | float | 当前 DL.P_EDC_cmd 值 (kW)，对应 IOA 16385 |
| `count` | int | 当前 DL.LFC_Count 值，脉冲步数累加器 |
| `cmd` | float | 当前 DL.LFC_cmd 值 (kW)，合成后的目标功率 |
| `pulse_status` | string | 脉冲状态机当前阶段 |

### pulse_status 枚举值

| 值 | 含义 |
|----|------|
| `INACTIVE` | 空闲，等待脉冲 |
| `PULSE_ON` | 脉冲 ON 阶段，每秒 count+1 |
| `IDLE_PERIOD` | 脉冲 OFF 空闲期，count 继续累加 |
| `HOLD` | 异常暂停（正负同时 ON 等） |

### 响应（实例无 pulse 数据，200）

```json
{
  "p_edc": 0.0,
  "count": 0,
  "cmd": 0.0,
  "pulse_status": "INACTIVE"
}
```

### 响应（实例不存在，404）

```json
{
  "error": "instance not found"
}
```

### 实现建议

- 直接读取实例中 IOA 16385（P_EDC）的当前值作为 `p_edc`
- count/cmd/pulse_status 从模拟器内部的脉冲处理状态中获取
- 如果实例未配置脉冲功能，返回默认零值

---

## 需求 2：batch-replay 补充 metrics_snapshot（P0）

### 用途

批量回放完成后，每个 CSV 的最终状态直接回传，无需额外查询。

### 现状

当前 `per_file_results` 只有状态：

```json
{
  "per_file_results": {
    "pulse_431_fwd.csv": {"status": "done"}
  }
}
```

### 需要改为

每条 CSV 回放完成时，把当时相关 IOA 的当前值写入 `metrics_snapshot`：

```json
{
  "per_file_results": {
    "pulse_431_fwd.csv": {
      "status": "done",
      "metrics_snapshot": {
        "count": 8,
        "cmd": 80.0,
        "p_edc": 0.0,
        "pulse_status": "IDLE_PERIOD"
      }
    },
    "pulse_431_rev.csv": {
      "status": "done",
      "metrics_snapshot": {
        "count": 8,
        "cmd": -80.0,
        "p_edc": 0.0,
        "pulse_status": "IDLE_PERIOD"
      }
    }
  }
}
```

### 实现方式

每条 CSV 回放完成时（`auto_cleanup` 的 sleep 2s 后），内部调用一次 `GET /metrics` 的逻辑（见需求 1），将结果存入 `metrics_snapshot`。

不需要新增 IOA 查询——直接用需求 1 的 metrics 结构。

---

## 需求 3：csv-replay 增加 loop 参数（P1）

### 用途

控制 CSV 是否循环播放。不回绕可避免脉冲信号无限重复干扰后续测试。

### 接口变更

`POST /api/v1/instances/{instance_id}/csv-replay` 请求体增加参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `loop` | bool | `true` (兼容旧行为) | `false`=播完最后一行后停止 |

### 请求示例

```json
{
  "csv_file": "pulse_431_fwd.csv",
  "time_format": "relative",
  "time_unit": "ms",
  "mappings": [
    {"column": 1, "ioa": 16386},
    {"column": 2, "ioa": 16387}
  ],
  "loop": false
}
```

### batch-replay 同步支持

`POST /api/v1/instances/{instance_id}/batch-replay` 请求体的 `loop` 参数作用于内部每条 csv-replay。

---

## 汇总

| # | API | 改动类型 | 优先级 | 工作量估计 |
|---|-----|---------|--------|-----------|
| 1 | `GET /metrics` | 新增接口 | P0 | ~30 行 |
| 2 | `batch-replay` 的 `metrics_snapshot` | 补充响应字段 | P0 | ~20 行 |
| 3 | `csv-replay` 的 `loop` | 新增请求参数 | P1 | ~10 行 |
