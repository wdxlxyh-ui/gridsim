---
title: 策略 - CSV 回放
tags:
  - strategy
  - csv
aliases:
  - CSV Replay Strategy
created: 2026-05-23
---

# CSV 回放策略

英文代号：`csv`

## 说明

按 CSV 文件定义的时间序列播放数据。支持单列和多列同步回放。

## 适用场景

回放真实录波数据，模拟历史场景或标准测试序列。

## 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `csv_file` | string | 是 | CSV 文件名 |
| `time_format` | string | 是 | `absolute`（绝对时间）或 `relative`（相对时间） |
| `time_unit` | string | 是 | `s`（秒）或 `ms`（毫秒），仅 relative 模式 |

## 计算逻辑

```
启动时:
  rows = parseCSV(csvPath)  // [{time, value}]
  
  if timeFormat == "absolute":
    按 hh:mm:ss 定位当前时刻→顺序播放到末尾停止
  
  if timeFormat == "relative":
    index = 0
    周期触发:
      row = rows[index]
      store.SetValue(ioa, row.value)
      publisher.Publish(point)
      time.Sleep(row.time)
      index = (index + 1) % len(rows)  // 循环
```

## CSV 文件查找路径

1. 优先查 `config/csv/{instanceID}/{filename}`
2. 回退到共享目录 `config/csv/{filename}`

> [!warning] 历史 Bug
> v2.1.5 之前引擎只查 `csv/{filename}`，与上传路径不匹配导致 CSV 回放失败。v2.2.0 已修复。

## 多测点同步回放

自 v2.5.0 起支持多列 CSV 文件同步回放，详见 [[功能特性/CSV多测点同步回放|CSV 多测点同步回放]]。

## 配置示例

```json
{
  "strategy": "csv",
  "enabled": true,
  "params": {
    "csv_file": "replay_data.csv",
    "time_format": "relative",
    "time_unit": "ms"
  }
}
```

---

相关笔记：[[功能特性/自动变化策略|自动变化策略总览]]、[[功能特性/CSV多测点同步回放|CSV 多测点同步回放]]
