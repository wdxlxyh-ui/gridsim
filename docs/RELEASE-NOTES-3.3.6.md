# GridSim v3.3.6 发布说明

- 标签：`v3.3.6`
- 日期：2026-08-18

## 趋势模板服务端持久化

- 趋势模板由浏览器 `localStorage` 迁移到 `config/trends/templates.json`。
- 新增模板 API：
  - `GET /api/v1/trend/templates`
  - `POST /api/v1/trend/templates`
  - `DELETE /api/v1/trend/templates/{id}`
- 支持覆盖保存、另存为和删除。
- 模板文件采用 `version` 字段，服务端校验模板名称、面板类型、实例 ID 和数量限制。
- 配置写入采用临时文件替换方式，减少写入中断导致文件损坏的风险。

## 兼容迁移

趋势页首次加载时，如果服务端尚无模板，会尝试迁移旧版浏览器缓存中的 `trend_templates`。迁移全部成功后删除旧缓存；部分迁移失败时保留兼容副本，不静默丢失模板。

当前浏览器的 `trend_panels` 仍用于保存当前工作区，不作为共享模板存储。

## 配置与日志目录

- 新增 `docs/配置与运行目录结构.md`。
- 共享趋势模板位于 `config/trends/templates.json`。
- SQLite 历史数据继续位于 `config/db/`。
- 实例日志现在真正使用 `--log-dir` 参数。
- Modbus 实例日志目录使用实际协议端口命名。
- 配置、历史数据库、点表备份、录制文件和运行日志分开管理。

## 升级注意事项

升级前请备份运行目录中的 `config/`。部署过程只替换程序、静态资源和默认目录，不应覆盖已存在的实例配置、趋势模板和历史数据库。
