# GridSim 打包结构规范

> **此文档为强制规范，任何打包脚本（Makefile / build.sh / CI）必须严格遵守。**
>
> 最后更新: 2026-06-14

---

## 标准发行包结构

### Linux (amd64 / arm64)

```
gridsim-v{VERSION}-linux-amd64/
├── bin/
│   ├── gridsim                    # 主程序二进制
│   ├── gridsim-mcp                # MCP Server 二进制（必含）
│   ├── VERSION                    # 版本号文件，内容为纯版本号字符串（必含）
│   ├── start.sh                   # 启动脚本（必含，位于 bin/ 内）
│   ├── stop.sh                    # 停止脚本（必含，位于 bin/ 内）
│   └── restart.sh                 # 重启脚本（必含，位于 bin/ 内）
├── config/
│   ├── instances.json             # 空实例配置 '[]'
│   └── users.json                 # 预置用户配置（必含）
├── logs/
│   └── .gitkeep
├── resources/
│   └── .gitkeep
├── web/dist/                      # Vue3 前端构建产物
│   ├── index.html
│   └── assets/
│       ├── css/
│       └── js/
└── samples/                       # 示例点表（可选）
    └── point.xlsx
```

### Windows (amd64)

```
gridsim-v{VERSION}-windows-amd64/
├── bin/
│   ├── gridsim.exe
│   ├── gridsim-mcp.exe
│   └── VERSION
├── scripts/                       # Windows: bat 脚本放 scripts/
│   ├── start.bat
│   ├── stop.bat
│   └── restart.bat
├── config/
│   ├── instances.json
│   └── users.json
├── logs/
│   └── .gitkeep
├── resources/
│   └── .gitkeep
└── web/dist/
```

---

## 关键规则

| 规则 | 说明 |
|---|---|
| **bin/ 统一存放** | Linux 包中，gridsim + gridsim-mcp + VERSION + 启停脚本全部在 `bin/` 目录 |
| **MCP 二进制必含** | 每个平台包必须包含对应架构的 `gridsim-mcp` |
| **VERSION 文件必含** | 位于 `bin/VERSION`，内容为版本号（如 `3.0.0`） |
| **users.json 必含** | 从 `config/users.json` 复制到包内 `config/users.json` |
| **resources/ 必含** | 即使只有 `.gitkeep`，目录必须存在 |
| **Linux 脚本位置** | `start.sh` / `stop.sh` / `restart.sh` 放在 `bin/` 内，**不是** `scripts/` |
| **Windows 脚本位置** | `start.bat` / `stop.bat` / `restart.bat` 放在 `scripts/` 内 |

---

## 打包方式

使用 `make dist` 命令打包（Makefile `dist` target 为唯一标准）：

```bash
# 完整打包（含前端构建 + 三平台）
make dist

# 输出：
#   dist/gridsim-v{VERSION}-linux-amd64.tar.gz
#   dist/gridsim-v{VERSION}-linux-arm64.tar.gz
#   dist/gridsim-v{VERSION}-windows-amd64.zip
```

> **警告**: `build.sh` 产出的包结构不完整（缺少 mcp、VERSION、users.json、resources，且脚本位置错误）。
> **不要使用 `build.sh` 作为最终发行打包工具。** 如需使用，必须先修正其打包逻辑以匹配上述结构。

---

## 打包前检查清单

- [ ] `bin/gridsim` 存在且可执行
- [ ] `bin/gridsim-mcp` 存在且可执行
- [ ] `bin/VERSION` 文件内容为正确版本号
- [ ] `bin/start.sh` / `stop.sh` / `restart.sh` 存在且有执行权限（Linux）
- [ ] `config/instances.json` 内容为 `[]`
- [ ] `config/users.json` 从源码 `config/users.json` 复制
- [ ] `logs/.gitkeep` 存在
- [ ] `resources/.gitkeep` 存在
- [ ] `web/dist/index.html` 存在（前端已构建）

---

## Makefile dist target 参考

标准打包逻辑定义在 `Makefile` 的 `dist` target（第 78-149 行），每个平台执行：

1. 创建目录结构: `bin/`, `config/`, `logs/`, `resources/`, `web/dist/`
2. 复制二进制: `gridsim` + `gridsim-mcp` → `bin/`
3. 复制版本号: `VERSION` → `bin/`
4. 复制脚本: `start.sh` / `stop.sh` / `restart.sh` → `bin/`（Linux）
5. 复制配置: `instances.json`(空) + `users.json` → `config/`
6. 复制前端: `web/dist/*` → `web/dist/`
7. 创建占位: `logs/.gitkeep` + `resources/.gitkeep`
8. 打包: `tar czf`（Linux）/ `zip`（Windows）
