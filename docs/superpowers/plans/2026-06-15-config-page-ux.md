# ConfigPage UX 改进实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 对 GridSim 配置管理界面进行 4 项 UX 改进：自动上传文件、覆盖重复文件、详情按钮、在线编辑点表

**Architecture:** 前端 Vue 3 + Element Plus 组件修改；后端 Go 新增 overwrite 参数和点表读写 API；xlsx 写回使用已有的 excelize 库

**Tech Stack:** Vue 3, TypeScript, Element Plus, Go 1.21+, excelize v2

---

## Task 1: 后端 — 上传文件支持 overwrite 参数

**Files:**
- Modify: `cmd/gridsim/main.go:647-715` (handleUpload)

- [ ] **Step 1: 修改 handleUpload 函数，支持 ?overwrite=true**

在 `cmd/gridsim/main.go` 的 `handleUpload` 函数中，在 `os.Stat(dst)` 检查之前，读取 `overwrite` 查询参数：

```go
overwrite := r.URL.Query().Get("overwrite") == "true"

if _, err := os.Stat(dst); err == nil {
    if !overwrite {
        writeError(w, http.StatusConflict, "file already exists")
        return
    }
    os.Remove(dst)
    slog.Info("覆盖已有文件", "filename", safeName, "uploader", r.RemoteAddr)
}
```

- [ ] **Step 2: 验证编译通过**

Run: `cd /root/IEC-SIM/iec104-sim-master && go build ./cmd/gridsim/`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add cmd/gridsim/main.go
git commit -m "feat(upload): support ?overwrite=true to replace existing files"
```

---

## Task 2: 前端 — uploadExcel 支持 overwrite + ConfigPage 自动上传

**Files:**
- Modify: `web/src/api/index.ts:104-111` (uploadExcel)
- Modify: `web/src/views/ConfigPage.vue:117-131` (upload area + footer)
- Modify: `web/src/views/ConfigPage.vue:313-331` (handleFileChange + handleUploadFirst)

- [ ] **Step 1: 修改 uploadExcel 函数，增加 overwrite 参数**

在 `web/src/api/index.ts` 中，修改 `uploadExcel` 函数签名和实现：

```typescript
export async function uploadExcel(file: File, overwrite = false): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post(`/upload?overwrite=${overwrite}`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.filename
}
```

- [ ] **Step 2: 修改 ConfigPage.vue — handleFileChange 改为自动上传**

在 `ConfigPage.vue` 的 script 中：

1. 新增 `uploading` ref：
```typescript
const uploading = ref(false)
```

2. 删除 `selectedFile` ref（L163）和 `handleUploadFirst` 函数（L317-331）

3. 替换 `handleFileChange` 函数（L313-315）：
```typescript
async function handleFileChange(file: any) {
  const raw = file.raw || file
  uploading.value = true
  try {
    const filename = await uploadExcel(raw, true)
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

4. 在 `resetForm` 中删除 `selectedFile.value = null`

- [ ] **Step 3: 修改 ConfigPage.vue — 删除"先上传文件"按钮和 selectedFile 显示**

在 template 中：

1. 删除 footer 中的"先上传文件"按钮（L126）：
```html
<!-- 删除这一行 -->
<el-button v-if="selectedFile" @click="handleUploadFirst" style="margin-right: 8px">先上传文件</el-button>
```

2. 修改"上传新文件"区域（L117-122），去掉 selectedFile 显示，加 loading：
```html
<el-form-item label="上传新文件" v-if="form.protocol !== 'microgrid'">
  <el-upload :auto-upload="false" :show-file-list="false" accept=".xlsx" :on-change="handleFileChange">
    <el-button type="primary" :loading="uploading">{{ uploading ? '上传中...' : '选择 Excel 文件' }}</el-button>
  </el-upload>
</el-form-item>
```

- [ ] **Step 4: 验证前端构建通过**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npx vue-tsc --noEmit && npx vite build`
Expected: 构建成功，无类型错误

- [ ] **Step 5: Commit**

```bash
git add web/src/api/index.ts web/src/views/ConfigPage.vue
git commit -m "feat(config): auto-upload on file select + overwrite support"
```

---

## Task 3: 前端 — 配置页增加"详情"按钮

**Files:**
- Modify: `web/src/views/ConfigPage.vue:56-69` (操作列)

- [ ] **Step 1: 在操作列中增加"详情"按钮**

在 `ConfigPage.vue` 的操作列 template 中，在"微电网"按钮之后、"编辑"按钮之前增加详情按钮：

```html
<el-button-group>
  <el-button v-if="row.status === 'running'" type="warning" size="small"
    :loading="actionLoading === row.id" @click="handleStop(row.id)">停止</el-button>
  <el-button v-else type="success" size="small"
    :loading="actionLoading === row.id" @click="handleStart(row.id)">启动</el-button>
  <el-button v-if="row.protocol === 'microgrid'" size="small" type="primary"
    @click="openMicrogrid(row.id)">微电网</el-button>
  <el-button size="small" @click="router.push('/detail/' + row.id)">详情</el-button>
  <el-button size="small" :disabled="row.status === 'running' || actionLoading === row.id" @click="handleEdit(row)">编辑</el-button>
  <el-button type="danger" size="small" :disabled="row.status === 'running' || actionLoading === row.id" @click="handleDelete(row.id)">删除</el-button>
</el-button-group>
```

- [ ] **Step 2: 调整操作列宽度**

将操作列宽度从 `280` 改为 `320`：

```html
<el-table-column label="操作" width="320" fixed="right">
```

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ConfigPage.vue
git commit -m "feat(config): add detail button in instance actions"
```

---

## Task 4: 后端 — 新增 xlsx 写回能力 (config.SaveToXLSX)

**Files:**
- Create: `pkg/config/writer.go`
- Create: `pkg/config/writer_test.go`

- [ ] **Step 1: 创建 writer.go — SaveToXLSX 函数**

创建 `pkg/config/writer.go`：

```go
package config

import (
	"fmt"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

// SaveToXLSX 将点表写回 xlsx 文件。
// 如果文件已存在则覆盖（清空 point 工作表后重写）。
func SaveToXLSX(points []*Point, path string) error {
	var f *excelize.File
	var err error

	// 尝试打开已有文件
	if _, err := excelize.OpenFile(path); err == nil {
		f, err = excelize.OpenFile(path)
		if err != nil {
			return fmt.Errorf("open existing xlsx: %w", err)
		}
		defer f.Close()
		// 删除 point 工作表
		f.DeleteSheet("point")
	} else {
		// 创建新文件
		f = excelize.NewFile()
		defer f.Close()
	}

	// 删除默认 Sheet1（如果是新文件）
	if idx, err := f.GetSheetIndex("Sheet1"); err == nil && idx >= 0 {
		f.DeleteSheet("Sheet1")
	}

	_, err = f.NewSheet("point")
	if err != nil {
		return fmt.Errorf("create sheet: %w", err)
	}

	// 写入表头
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue("point", cell, h)
	}

	// 写入点表行
	for i, p := range points {
		row := i + 2
		f.SetCellValue("point", axis("A", row), p.Name)
		f.SetCellValue("point", axis("B", row), int(p.IOA))
		f.SetCellValue("point", axis("C", row), string(p.ValueType))
		f.SetCellValue("point", axis("D", row), string(p.PointType))
		f.SetCellValue("point", axis("E", row), p.Efficient)
		f.SetCellValue("point", axis("F", row), p.BaseValue)
		f.SetCellValue("point", axis("G", row), p.Alias)

		// Modbus 扩展列
		if p.FunctionCode > 0 {
			f.SetCellValue("point", axis("H", row), int(p.RegisterAddress))
			f.SetCellValue("point", axis("I", row), int(p.FunctionCode))
		}
	}

	// 确保输出目录存在
	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("save xlsx: %w", err)
	}

	return nil
}

func axis(col string, row int) string {
	cell, _ := excelize.CoordinatesToCellName(colToIndex(col), row)
	return cell
}

func colToIndex(col string) int {
	idx := 0
	for _, c := range col {
		idx = idx*26 + int(c-'A') + 1
	}
	return idx
}

// TempSaveToXLSX 先写到临时文件再 rename，保证原子性
func TempSaveToXLSX(points []*Point, path string) error {
	dir := filepath.Dir(path)
	tmpPath := filepath.Join(dir, ".tmp_"+filepath.Base(path))

	if err := SaveToXLSX(points, tmpPath); err != nil {
		return err
	}

	return renameFile(tmpPath, path)
}

func renameFile(oldPath, newPath string) error {
	// 使用标准库重命名
	return os.Rename(oldPath, newPath)
}
```

注意：需要 import "os" 包。在文件顶部的 import 中加入 `"os"`。

- [ ] **Step 2: 创建 writer_test.go — SaveToXLSX 测试**

创建 `pkg/config/writer_test.go`：

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xlsx")

	original := []*Point{
		{IOA: 16385, Name: "母线电压", ValueType: VTFloat, PointType: TypeAI, Efficient: 1.0, BaseValue: 220.0, Alias: "Bus-V"},
		{IOA: 5, Name: "开关1", ValueType: VTBit, PointType: TypeDI, Efficient: 1.0, BaseValue: 0, Alias: ""},
		{IOA: 100, Name: "脉冲1", ValueType: VTInt, PointType: TypePI, Efficient: 1.0, BaseValue: 100, Alias: ""},
	}

	err := SaveToXLSX(original, path)
	assert.NoError(t, err)
	assert.FileExists(t, path)

	loaded, err := LoadFromXLSX(path, "iec104")
	assert.NoError(t, err)
	assert.Len(t, loaded, 3)

	assert.Equal(t, "母线电压", loaded[0].Name)
	assert.Equal(t, uint32(16385), loaded[0].IOA)
	assert.Equal(t, TypeAI, loaded[0].PointType)
	assert.Equal(t, 220.0, loaded[0].BaseValue)

	// 覆盖写入
	modified := []*Point{
		{IOA: 16385, Name: "修改后的电压", ValueType: VTFloat, PointType: TypeAI, Efficient: 1.5, BaseValue: 230.0, Alias: "Modified"},
	}
	err = SaveToXLSX(modified, path)
	assert.NoError(t, err)

	loaded2, err := LoadFromXLSX(path, "iec104")
	assert.NoError(t, err)
	assert.Len(t, loaded2, 1)
	assert.Equal(t, "修改后的电压", loaded2[0].Name)
}

func TestSaveToXLSXModbus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modbus_test.xlsx")

	points := []*Point{
		{IOA: 1, Name: "线圈1", ValueType: VTBit, PointType: TypeDI, Efficient: 1.0, BaseValue: 0, Alias: "",
			FunctionCode: 1, RegisterAddress: 100},
		{IOA: 2, Name: "温度", ValueType: VTFloat, PointType: TypeAI, Efficient: 0.1, BaseValue: 250, Alias: "Temp",
			FunctionCode: 3, RegisterAddress: 200},
	}

	err := SaveToXLSX(points, path)
	assert.NoError(t, err)

	loaded, err := LoadFromXLSX(path, "modbus_tcp")
	assert.NoError(t, err)
	assert.Len(t, loaded, 2)
	assert.Equal(t, uint8(3), loaded[1].FunctionCode)
	assert.Equal(t, uint16(200), loaded[1].RegisterAddress)
}
```

注意：需要检查项目中是否有 testify 依赖。如果没有，改用 `if err != nil { t.Fatal(err) }` 风格。检查：`grep testify go.mod`。

- [ ] **Step 3: 检查 testify 依赖，运行测试**

Run: `cd /root/IEC-SIM/iec104-sim-master && grep testify go.mod`
如果不存在，需要先安装：`go get github.com/stretchr/testify`

Run: `cd /root/IEC-SIM/iec104-sim-master && go test ./pkg/config/ -v -run TestSave`
Expected: 所有测试通过

- [ ] **Step 4: Commit**

```bash
git add pkg/config/writer.go pkg/config/writer_test.go
git commit -m "feat(config): add SaveToXLSX for writing point table back to xlsx"
```

---

## Task 5: 后端 — 新增点表读取/保存 API

**Files:**
- Modify: `cmd/gridsim/main.go` (路由注册 + handler)

- [ ] **Step 1: 新增 handlePointTableGet 和 handlePointTablePut 函数**

在 `cmd/gridsim/main.go` 中添加两个 handler 函数。

**GET handler** — 读取点表：

```go
func (ws *webServer) handlePointTableGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := r.URL.Path[len("/api/v1/instances/"):]
	// 去掉 /point-table 后缀
	id = strings.TrimSuffix(id, "/point-table")

	state := ws.mgr.GetState(id)
	if state == nil {
		writeError(w, http.StatusNotFound, "instance not found")
		return
	}
	if state.Status == model.StatusRunning {
		writeError(w, http.StatusBadRequest, "cannot edit point table while instance is running")
		return
	}

	cfg := ws.mgr.GetConfig(id)
	if cfg == nil {
		writeError(w, http.StatusNotFound, "instance config not found")
		return
	}

	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		xlsxPath = filepath.Join(ws.cfgDir, xlsxPath)
	}

	points, err := config.LoadFromXLSX(xlsxPath, cfg.Protocol)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load xlsx: "+err.Error())
		return
	}

	// 序列化为 JSON 格式
	type pointRow struct {
		Name           string  `json:"name"`
		IOA            uint32  `json:"ioa"`
		ValueType      string  `json:"value_type"`
		PointType      string  `json:"point_type"`
		Efficient      float64 `json:"efficient"`
		BaseValue      float64 `json:"base_value"`
		Alias          string  `json:"alias"`
		FunctionCode   uint8   `json:"function_code,omitempty"`
		RegisterAddress uint16  `json:"register_address,omitempty"`
	}

	rows := make([]pointRow, len(points))
	for i, p := range points {
		rows[i] = pointRow{
			Name: p.Name, IOA: p.IOA, ValueType: string(p.ValueType),
			PointType: string(p.PointType), Efficient: p.Efficient,
			BaseValue: p.BaseValue, Alias: p.Alias,
			FunctionCode: p.FunctionCode, RegisterAddress: p.RegisterAddress,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"xlsx_file": cfg.XLSXFile,
		"points":    rows,
	})
}
```

**PUT handler** — 保存点表：

```go
func (ws *webServer) handlePointTablePut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := r.URL.Path[len("/api/v1/instances/"):]
	id = strings.TrimSuffix(id, "/point-table")

	state := ws.mgr.GetState(id)
	if state == nil {
		writeError(w, http.StatusNotFound, "instance not found")
		return
	}
	if state.Status == model.StatusRunning {
		writeError(w, http.StatusBadRequest, "cannot edit point table while instance is running")
		return
	}

	cfg := ws.mgr.GetConfig(id)
	if cfg == nil {
		writeError(w, http.StatusNotFound, "instance config not found")
		return
	}

	var req struct {
		Points []struct {
			Name           string  `json:"name"`
			IOA            uint32  `json:"ioa"`
			ValueType      string  `json:"value_type"`
			PointType      string  `json:"point_type"`
			Efficient      float64 `json:"efficient"`
			BaseValue      float64 `json:"base_value"`
			Alias          string  `json:"alias"`
			FunctionCode   uint8   `json:"function_code"`
			RegisterAddress uint16  `json:"register_address"`
		} `json:"points"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(req.Points) == 0 {
		writeError(w, http.StatusBadRequest, "points array cannot be empty")
		return
	}

	// 转换为 config.Point
	points := make([]*config.Point, len(req.Points))
	seen := make(map[string]bool)
	for i, rp := range req.Points {
		pt := config.PointType(rp.PointType)
		vt := config.ValueType(rp.ValueType)
		if !isValidPointType(pt) || !isValidValueType(vt) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid type at row %d", i+1))
			return
		}
		key := string(pt) + ":" + fmt.Sprint(rp.IOA)
		if seen[key] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("duplicate %s IOA %d", pt, rp.IOA))
			return
		}
		seen[key] = true

		p := &config.Point{
			IOA: rp.IOA, Name: rp.Name, ValueType: vt, PointType: pt,
			Efficient: rp.Efficient, BaseValue: rp.BaseValue, Alias: rp.Alias,
			FunctionCode: rp.FunctionCode, RegisterAddress: rp.RegisterAddress,
		}
		// 设置初始值
		switch pt {
		case config.TypeAI, config.TypeAO:
			p.Value = rp.BaseValue * rp.Efficient
		case config.TypeDI, config.TypeDO:
			p.BoolValue = rp.BaseValue != 0
		case config.TypePI:
			p.IntValue = int32(rp.BaseValue)
		}
		points[i] = p
	}

	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		xlsxPath = filepath.Join(ws.cfgDir, xlsxPath)
	}

	if err := config.TempSaveToXLSX(points, xlsxPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save xlsx: "+err.Error())
		return
	}

	slog.Info("点表已更新", "instance", id, "points", len(points), "xlsx", cfg.XLSXFile)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "xlsx_file": cfg.XLSXFile})
}

func isValidPointType(pt config.PointType) bool {
	switch pt {
	case config.TypeAI, config.TypeDI, config.TypePI, config.TypeDO, config.TypeAO:
		return true
	}
	return false
}

func isValidValueType(vt config.ValueType) bool {
	switch vt {
	case config.VTFloat, config.VTDouble, config.VTInt, config.VTBit:
		return true
	}
	return false
}
```

注意：需要检查 `ws.mgr` 是否有 `GetConfig` 方法。如果没有，使用 `ws.mgr.GetState(id)` 返回的状态对象中的字段，或者从 storage 读取。需要查看 `internal/manager/manager.go` 的接口。

- [ ] **Step 2: 在 handleInstanceByID 的 switch 中添加 point-table 路由**

在 `cmd/gridsim/main.go` 的 `handleInstanceByID` 函数中，找到 `switch parts[1]` 语句，添加新 case：

```go
case "point-table":
    switch r.Method {
    case http.MethodGet:
        ws.handlePointTableGet(w, r, id)
    case http.MethodPut:
        ws.handlePointTablePut(w, r, id)
    default:
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
    }
```

注意：不需要新增路由注册或分发函数，直接复用现有的 `/api/v1/instances/` 路由和 `handleInstanceByID` 的路径解析。

同时更新 handler 函数签名，接收 `id string` 参数而不是从 URL 解析：

```go
func (ws *webServer) handlePointTableGet(w http.ResponseWriter, r *http.Request, id string) { ... }
func (ws *webServer) handlePointTablePut(w http.ResponseWriter, r *http.Request, id string) { ... }
```

handler 内部使用 `ws.mgr.GetConfig(id)` 获取配置（注意：`GetConfig` 返回 `(model.InstanceConfig, bool)`），使用 `ws.mgr.GetState(id)` 获取状态并检查 `state.Status != model.StatusRunning`。

配置中的 xlsx 路径通过 `cfg.XLSXFile` 获取，如果非绝对路径则拼接 `ws.cfgDir`。

Run: `cd /root/IEC-SIM/iec104-sim-master && go build ./cmd/gridsim/`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add cmd/gridsim/main.go
git commit -m "feat(api): add point-table GET/PUT endpoints for online editing"
```

---

## Task 6: 前端 — 新增 PointTableEditor 组件

**Files:**
- Create: `web/src/components/PointTableEditor.vue`

- [ ] **Step 1: 创建 PointTableEditor.vue 组件**

创建 `web/src/components/PointTableEditor.vue`：

```vue
<template>
  <el-dialog v-model="dialogVisible" title="编辑点表" width="900px" @open="loadPoints" :close-on-click-modal="false">
    <div v-loading="loading" style="min-height: 200px">
      <el-alert v-if="duplicateIOAs.length > 0" type="error" :closable="false"
        :title="'IOA 重复: ' + duplicateIOAs.join(', ')" style="margin-bottom: 12px" />

      <el-table :data="editPoints" border stripe style="width: 100%" max-height="500"
        @selection-change="onSelectionChange">
        <el-table-column type="selection" width="40" />
        <el-table-column label="IOA" width="100">
          <template #default="{ row }">
            <el-input-number v-model="row.ioa" :min="0" :max="16777215" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column label="名称" min-width="140">
          <template #default="{ row }">
            <el-input v-model="row.name" size="small" />
          </template>
        </el-table-column>
        <el-table-column label="数据类型" width="120">
          <template #default="{ row }">
            <el-select v-model="row.value_type" size="small" style="width: 100%">
              <el-option label="FLOAT" value="FLOAT" />
              <el-option label="DOUBLE" value="DOUBLE" />
              <el-option label="INT" value="INT" />
              <el-option label="BIT" value="BIT" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="测点类型" width="110">
          <template #default="{ row }">
            <el-select v-model="row.point_type" size="small" style="width: 100%">
              <el-option label="AI (遥测)" value="AI" />
              <el-option label="DI (遥信)" value="DI" />
              <el-option label="PI (遥脉)" value="PI" />
              <el-option label="DO (遥控)" value="DO" />
              <el-option label="AO (遥调)" value="AO" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="系数" width="90">
          <template #default="{ row }">
            <el-input-number v-model="row.efficient" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column label="基值" width="90">
          <template #default="{ row }">
            <el-input-number v-model="row.base_value" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column label="别名" width="100">
          <template #default="{ row }">
            <el-input v-model="row.alias" size="small" />
          </template>
        </el-table-column>
        <el-table-column v-if="isModbus" label="寄存器地址" width="110">
          <template #default="{ row }">
            <el-input-number v-model="row.register_address" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column v-if="isModbus" label="功能码" width="90">
          <template #default="{ row }">
            <el-select v-model="row.function_code" size="small" style="width: 100%">
              <el-option :label="1" :value="1" />
              <el-option :label="3" :value="3" />
              <el-option :label="4" :value="4" />
              <el-option :label="16" :value="16" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="60" fixed="right">
          <template #default="{ $index }">
            <el-button type="danger" size="small" :icon="Delete" circle @click="removeRow($index)" />
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 12px; font-size: 12px; color: #999">
        共 {{ editPoints.length }} 个测点
      </div>
    </div>
    <template #footer>
      <el-button @click="dialogVisible = false" :disabled="saving">取消</el-button>
      <el-button type="warning" @click="addRow" :disabled="saving">新增行</el-button>
      <el-button type="danger" @click="deleteSelected" :disabled="saving || selectedRows.length === 0">
        删除选中 ({{ selectedRows.length }})
      </el-button>
      <el-button type="primary" @click="save" :loading="saving" :disabled="duplicateIOAs.length > 0">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const props = defineProps<{
  visible: boolean
  instanceId: string
  protocol?: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'saved'): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const loading = ref(false)
const saving = ref(false)
const editPoints = ref<any[]>([])
const selectedRows = ref<any[]>([])

const isModbus = computed(() => props.protocol === 'modbus_tcp')

const duplicateIOAs = computed(() => {
  const seen = new Map<number, string[]>()
  for (const p of editPoints.value) {
    const key = `${p.point_type}:${p.ioa}`
    if (!seen.has(key)) seen.set(key, [])
    seen.get(key)!.push(p.name)
  }
  const dups: string[] = []
  for (const [key, names] of seen) {
    if (names.length > 1) dups.push(`${key} (${names.join(', ')})`)
  }
  return dups
})

async function loadPoints() {
  loading.value = true
  try {
    const resp = await fetch(`/api/v1/instances/${props.instanceId}/point-table`)
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}))
      throw new Error(err?.error?.message || err?.error || '加载失败')
    }
    const data = await resp.json()
    editPoints.value = data.points.map((p: any) => ({
      ...p,
      register_address: p.register_address || 0,
      function_code: p.function_code || 0,
    }))
  } catch (e: any) {
    ElMessage.error('加载点表失败: ' + e.message)
    dialogVisible.value = false
  } finally {
    loading.value = false
  }
}

function addRow() {
  editPoints.value.push({
    ioa: 0,
    name: '',
    value_type: 'FLOAT',
    point_type: 'AI',
    efficient: 1.0,
    base_value: 0,
    alias: '',
    register_address: 0,
    function_code: 0,
  })
}

function removeRow(index: number) {
  editPoints.value.splice(index, 1)
}

function onSelectionChange(rows: any[]) {
  selectedRows.value = rows
}

function deleteSelected() {
  const ids = new Set(selectedRows.value)
  editPoints.value = editPoints.value.filter(p => !ids.has(p))
  selectedRows.value = []
}

async function save() {
  saving.value = true
  try {
    const resp = await fetch(`/api/v1/instances/${props.instanceId}/point-table`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ points: editPoints.value }),
    })
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}))
      throw new Error(err?.error?.message || err?.error || '保存失败')
    }
    ElMessage.success('点表已保存')
    emit('saved')
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}
</script>
```

注意：组件直接使用 `fetch` 而非 axios，避免与 `api/index.ts` 的 baseURL 冲突（`/api/v1` baseURL 会在路径前再加前缀导致双前缀）。或者可以在 `api/index.ts` 中新增两个函数供组件调用。

- [ ] **Step 2: 在 api/index.ts 中新增点表 API 函数**

```typescript
export interface PointTableRow {
  name: string
  ioa: number
  value_type: string
  point_type: string
  efficient: number
  base_value: number
  alias: string
  function_code?: number
  register_address?: number
}

export interface PointTableResponse {
  xlsx_file: string
  points: PointTableRow[]
}

export async function getPointTable(instanceId: string): Promise<PointTableResponse> {
  const res = await http.get(`/instances/${instanceId}/point-table`)
  return res.data
}

export async function savePointTable(instanceId: string, points: PointTableRow[]): Promise<void> {
  await http.put(`/instances/${instanceId}/point-table`, { points })
}
```

- [ ] **Step 3: 更新 PointTableEditor.vue 使用 api 函数**

将组件中的 `fetch` 调用替换为 `getPointTable` 和 `savePointTable`：

```typescript
import { getPointTable, savePointTable, type PointTableRow } from '../api'

async function loadPoints() {
  loading.value = true
  try {
    const data = await getPointTable(props.instanceId)
    editPoints.value = data.points.map(p => ({
      ...p,
      register_address: p.register_address || 0,
      function_code: p.function_code || 0,
    }))
  } catch (e: any) {
    ElMessage.error('加载点表失败: ' + (e?.response?.data?.error || e.message))
    dialogVisible.value = false
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await savePointTable(props.instanceId, editPoints.value)
    ElMessage.success('点表已保存')
    emit('saved')
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}
```

- [ ] **Step 4: 验证前端构建通过**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npx vue-tsc --noEmit && npx vite build`
Expected: 构建成功

- [ ] **Step 5: Commit**

```bash
git add web/src/components/PointTableEditor.vue web/src/api/index.ts
git commit -m "feat(config): add PointTableEditor component for online point table editing"
```

---

## Task 7: 前端 — ConfigPage 和 DetailPage 集成点表编辑按钮

**Files:**
- Modify: `web/src/views/ConfigPage.vue` (导入组件 + 编辑点表按钮 + 状态)
- Modify: `web/src/views/DetailPage.vue` (导入组件 + 编辑点表按钮)

- [ ] **Step 1: ConfigPage.vue — 导入组件并增加编辑点表按钮**

在 `ConfigPage.vue` 中：

1. 导入组件：
```typescript
import PointTableEditor from '../components/PointTableEditor.vue'
```

2. 新增状态：
```typescript
const pointEditorVisible = ref(false)
const pointEditorInstanceId = ref('')
const pointEditorProtocol = ref('')
```

3. 新增打开函数：
```typescript
function openPointTableEditor(id: string, protocol?: string) {
  pointEditorInstanceId.value = id
  pointEditorProtocol.value = protocol || 'iec104'
  pointEditorVisible.value = true
}
```

4. 在 template 末尾（`</el-dialog>` 之后）添加组件：
```html
<PointTableEditor
  v-model:visible="pointEditorVisible"
  :instance-id="pointEditorInstanceId"
  :protocol="pointEditorProtocol"
  @saved="fetchData"
/>
```

5. 在操作列中增加"编辑点表"按钮（在"详情"按钮之后）：
```html
<el-button size="small" @click="openPointTableEditor(row.id, row.protocol)"
  :disabled="row.status === 'running'">编辑点表</el-button>
```

6. 调整操作列宽度为 `380` 以容纳所有按钮。

- [ ] **Step 2: DetailPage.vue — 导入组件并增加编辑点表按钮**

在 `DetailPage.vue` 中：

1. 导入组件：
```typescript
import PointTableEditor from '../components/PointTableEditor.vue'
```

2. 新增状态：
```typescript
const pointEditorVisible = ref(false)
```

3. 在顶部卡片的操作区域（右侧，polling 控件之前）增加按钮：
```html
<el-button v-if="instanceStatus !== 'running'" size="small" type="warning" @click="pointEditorVisible = true">编辑点表</el-button>
```

4. 在 template 末尾添加组件：
```html
<PointTableEditor
  v-model:visible="pointEditorVisible"
  :instance-id="instanceId"
  :protocol="instanceProtocol"
  @saved="refreshPoints"
/>
```

注意：需要确认 DetailPage 中的 `instanceId` 变量名和 `instanceProtocol` 是否存在。需要在 DetailPage 中查找当前实例配置中 protocol 的获取方式。

- [ ] **Step 3: 验证前端构建通过**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npx vue-tsc --noEmit && npx vite build`
Expected: 构建成功

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ConfigPage.vue web/src/views/DetailPage.vue
git commit -m "feat(config): integrate point table editor in ConfigPage and DetailPage"
```

---

## Task 8: 集成测试与构建验证

**Files:**
- No new files

- [ ] **Step 1: 运行后端测试**

Run: `cd /root/IEC-SIM/iec104-sim-master && go test ./pkg/... -v`
Expected: 所有测试通过

- [ ] **Step 2: 全量构建**

Run: `cd /root/IEC-SIM/iec104-sim-master && make dist`
Expected: 三平台构建成功，前端构建无错误

- [ ] **Step 3: 验证 API 端点**

启动服务后手动测试（或 curl）：

```bash
# 1. 测试覆盖上传
curl -X POST http://localhost:8989/api/v1/upload?overwrite=true \
  -F "file=@test.xlsx" -H "Cookie: token=..."

# 2. 测试读取点表
curl http://localhost:8989/api/v1/instances/{id}/point-table -H "Cookie: token=..."

# 3. 测试保存点表
curl -X PUT http://localhost:8989/api/v1/instances/{id}/point-table \
  -H "Content-Type: application/json" -H "Cookie: token=..." \
  -d '{"points":[{"name":"test","ioa":1,"value_type":"FLOAT","point_type":"AI","efficient":1,"base_value":0,"alias":""}]}'
```

- [ ] **Step 4: Final commit（如有修复）**

如有集成测试发现的问题，修复后提交。
