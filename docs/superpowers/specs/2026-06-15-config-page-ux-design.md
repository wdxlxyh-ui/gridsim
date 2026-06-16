# ConfigPage UX 改进设计

> 日期: 2026-06-15 | 版本: v3.0.2 | 状态: 设计中

## 1. 概述

对 GridSim 配置管理界面进行 4 项 UX 改进，减少用户操作步骤，提升日常使用效率。

| # | 需求 | 复杂度 | 改动范围 |
|---|------|--------|----------|
| 1 | 编辑实例时选择文件后自动上传 | 低 | 前端 |
| 2 | 文件名重复时覆盖旧文件 | 低 | 前端 + 后端 |
| 3 | 配置页操作列增加"详情"按钮 | 极低 | 前端 |
| 4 | 停用实例时在线编辑点表 | 中高 | 前端 + 后端 |

---

## 2. 需求 1：选择文件后自动上传

### 现状

`ConfigPage.vue` 的编辑实例弹窗：
- 用户选择 Excel 文件后，仅将文件暂存到 `selectedFile` ref
- 需要手动点击 footer 中的 "先上传文件" 按钮
- 上传成功后文件出现在下拉框中
- 然后选择下拉框中的文件，最后点 "创建/保存"

**操作步骤：选文件 → 点"先上传文件" → 选下拉框 → 点保存 = 4 步**

### 方案

修改 `handleFileChange`，选择文件后立即调用 `uploadExcel`，上传成功后自动设置 `form.xlsx_file` 并刷新文件列表。

**操作步骤：选文件 → 自动上传并选中 → 点保存 = 2 步**

### 具体改动

**`web/src/views/ConfigPage.vue`**:

```typescript
// handleFileChange 改为 async 自动上传
async function handleFileChange(file: any) {
  const raw = file.raw || file
  uploading.value = true
  try {
    const filename = await uploadExcel(raw, true) // overwrite=true
    ElMessage.success('上传成功: ' + filename)
    form.value.xlsx_file = filename
    await fetchFiles()
  } catch (e: any) {
    ElMessage.error('上传失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    uploading.value = false
  }
}
```

- 删除 footer 中的 "先上传文件" 按钮（L126）
- 删除 `selectedFile` ref 和 `handleUploadFirst` 函数
- `el-upload` 的 `on-change` 保持调用 `handleFileChange`
- 新增 `uploading` ref 控制上传 loading 状态，在按钮上显示

**`web/src/api/index.ts`**:

```typescript
// uploadExcel 增加 overwrite 参数
export async function uploadExcel(file: File, overwrite = false): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post(`/upload?overwrite=${overwrite}`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.filename
}
```

### 边界条件

- 上传中禁止关闭弹窗或重复上传（`uploading` loading 状态）
- 上传失败不改变当前已选文件，用户可以重新选择

---

## 3. 需求 2：文件名重复时覆盖旧文件

### 现状

`cmd/gridsim/main.go` L695-698：

```go
if _, err := os.Stat(dst); err == nil {
    writeError(w, http.StatusConflict, "file already exists")
    return
}
```

同名文件直接返回 409 错误。

### 方案

后端支持 `?overwrite=true` 查询参数。前端自动上传时始终带此参数（与需求 1 联动）。

### 具体改动

**`cmd/gridsim/main.go`** — `handleUpload` 函数：

```go
// 在 os.Stat 检查之前
overwrite := r.URL.Query().Get("overwrite") == "true"

if _, err := os.Stat(dst); err == nil {
    if !overwrite {
        writeError(w, http.StatusConflict, "file already exists")
        return
    }
    os.Remove(dst) // 覆盖模式：先删除旧文件
}
```

### 安全考虑

- 仅当显式传入 `overwrite=true` 时才覆盖，防止意外
- 后端记录覆盖操作的日志（slog.Info 标注 overwrite）

---

## 4. 需求 3：配置页增加"详情"按钮

### 现状

`ConfigPage.vue` 操作列 L56-69：启动/停止、微电网、编辑、删除

### 方案

在 `el-button-group` 中增加 "详情" 按钮，跳转 `/detail/:id`。**始终可用**（不论实例运行状态）。

### 具体改动

**`web/src/views/ConfigPage.vue`** — 操作列模板：

```html
<el-button-group>
  <el-button v-if="row.status === 'running'" type="warning" size="small" ...>停止</el-button>
  <el-button v-else type="success" size="small" ...>启动</el-button>
  <el-button v-if="row.protocol === 'microgrid'" size="small" type="primary" ...>微电网</el-button>
  <el-button size="small" @click="router.push('/detail/' + row.id)">详情</el-button>  <!-- 新增 -->
  <el-button size="small" :disabled="row.status === 'running' || actionLoading === row.id" ...>编辑</el-button>
  <el-button type="danger" size="small" :disabled="row.status === 'running' || actionLoading === row.id" ...>删除</el-button>
</el-button-group>
```

- 详情按钮放在"微电网"和"编辑"之间
- 无需 disabled 逻辑，任何状态都可以查看详情
- 操作列宽度从 280px 调整为 320px

---

## 5. 需求 4：在线编辑点表

### 现状

修改点表需要：下载 xlsx → 本地用 Excel 修改 → 重新上传 → 重启实例。流程繁琐。

### 方案

新增独立的**点表编辑弹窗**，可从两个入口打开：
- **配置页 (ConfigPage)**：操作列"编辑点表"按钮（仅 stopped 状态）
- **详情页 (DetailPage)**：顶部操作栏"编辑点表"按钮（仅 stopped 状态）

弹窗内显示可编辑的 `el-table`，支持增删改点表行。保存后同步更新 xlsx 文件。

### 5.1 后端改动

#### 新增 API：读取点表

```
GET /api/v1/instances/{id}/point-table
```

从实例关联的 xlsx 文件读取点表，返回 JSON 数组。

**响应**:
```json
{
  "xlsx_file": "station-a.xlsx",
  "points": [
    {
      "name": "母线电压",
      "ioa": 16385,
      "value_type": "FLOAT",
      "point_type": "AI",
      "efficient": 1.0,
      "base_value": 220.0,
      "alias": "Bus-V"
    }
  ]
}
```

**实现**：复用 `config.LoadFromXLSX(path, protocol)`，从实例配置的 `xlsx_file` 构造路径。

**约束**：实例必须为 `stopped` 状态，否则返回 400。

#### 新增 API：保存点表

```
PUT /api/v1/instances/{id}/point-table
```

接收完整点表 JSON，校验后写回 xlsx 文件。

**请求体**:
```json
{
  "points": [
    { "name": "母线电压", "ioa": 16385, "value_type": "FLOAT", "point_type": "AI", "efficient": 1.0, "base_value": 220.0, "alias": "" },
    { "name": "开关1", "ioa": 5, "value_type": "BIT", "point_type": "DI", "efficient": 1.0, "base_value": 0, "alias": "" }
  ]
}
```

**实现**：
1. 校验实例为 stopped
2. 校验 points 数组（IOA 唯一、类型合法）
3. 调用新增的 `config.SaveToXLSX(points, path)` 写回文件
4. 返回 200 成功

#### 新增：xlsx 写回能力

**`pkg/config/writer.go`（新文件）**:

```go
func SaveToXLSX(points []*Point, path string) error
```

使用 `excelize` 库：
- 创建新文件或打开已有文件
- 清空 `point` 工作表
- 写入表头行（point-name, point-number, value-type, point-type, efficient, base-value, alias）
- 按顺序写入所有 point 行
- 保存文件

**写入格式**与 `LoadFromXLSX` 读取格式完全对应，保证读写一致性。

### 5.2 前端改动

#### 点表编辑弹窗组件

**`web/src/components/PointTableEditor.vue`（新组件）**：

Props:
- `visible: boolean` — 控制弹窗显示
- `instanceId: string` — 实例 ID

功能：
- 弹窗打开时调用 `GET /api/v1/instances/{id}/point-table` 加载点表
- `el-table` 显示点表行，使用 `el-input` / `el-select` 实现行内编辑
- 底部操作栏：
  - "新增行" 按钮 — 在表格末尾追加空行
  - "删除选中" 按钮 — 批量删除勾选行
  - "保存" 按钮 — 调用 `PUT /api/v1/instances/{id}/point-table`
  - "取消" 按钮 — 关闭弹窗不保存
- IOA 列重复高亮（红色提示）
- 保存前前端校验：必填字段、IOA 唯一、类型合法

**表格列**：

| 列 | 字段 | 编辑方式 |
|----|------|----------|
| IOA | ioa | el-input-number |
| 名称 | name | el-input |
| 数据类型 | value_type | el-select (FLOAT/DOUBLE/INT/BIT) |
| 测点类型 | point_type | el-select (AI/DI/PI/DO/AO) |
| 系数 | efficient | el-input-number |
| 基值 | base_value | el-input-number |
| 别名 | alias | el-input |
| 寄存器地址 | register_address | el-input-number (仅 Modbus) |
| 功能码 | function_code | el-select (仅 Modbus) |
| 操作 | - | 删除按钮 |

#### 入口按钮

**ConfigPage.vue** — 操作列新增"编辑点表"按钮（仅在 stopped 状态显示）：

```html
<el-button size="small" @click="openPointTableEditor(row.id)"
  :disabled="row.status === 'running'">编辑点表</el-button>
```

**DetailPage.vue** — 顶部操作栏新增"编辑点表"按钮（仅在 stopped 状态显示）。

### 5.3 数据流

```
用户打开弹窗 → GET point-table → xlsx文件 → JSON点表 → el-table显示
                                                     ↓
用户编辑点表 → PUT point-table → 校验 → config.SaveToXLSX → 覆盖xlsx文件
                                                     ↓
下次启动实例 → config.LoadFromXLSX → 加载新点表到内存
```

### 5.4 边界条件

- 实例 running 时禁止编辑（后端返回 400）
- xlsx 文件不存在时返回 404
- Modbus 点表需要额外列（register-address, function-code），编辑时按协议类型显示/隐藏对应列
- 保存失败不改变 xlsx 文件（先写临时文件再 rename）
- 编辑弹窗有 loading 状态（加载中 / 保存中禁止操作）

---

## 6. 文件改动汇总

| 文件 | 操作 | 需求 |
|------|------|------|
| `web/src/views/ConfigPage.vue` | 修改 | #1 #2 #3 #4 |
| `web/src/views/DetailPage.vue` | 修改 | #4 |
| `web/src/api/index.ts` | 修改 | #1 #4 |
| `web/src/components/PointTableEditor.vue` | **新增** | #4 |
| `cmd/gridsim/main.go` | 修改 | #2 #4 |
| `pkg/config/writer.go` | **新增** | #4 |

---

## 7. 不在范围内

- 点表编辑不支持导入/导出（已有 `exportPointsCSV` 功能）
- 不支持多实例同时编辑同一个 xlsx 文件的并发控制（单用户场景足够）
