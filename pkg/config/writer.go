package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

// SaveToXLSX 将点表写回 xlsx 文件。
// 如果文件已存在则覆盖（清空 point 工作表后重写）。
func SaveToXLSX(points []*Point, path string) error {
	var f *excelize.File
	isNew := true

	// 尝试打开已有文件
	if _, err := os.Stat(path); err == nil {
		var openErr error
		f, openErr = excelize.OpenFile(path)
		if openErr != nil {
			return fmt.Errorf("open existing xlsx: %w", openErr)
		}
		isNew = false
		// 删除 point 工作表以便重写
		f.DeleteSheet("point")
	} else {
		f = excelize.NewFile()
	}

	// 新文件：删除默认 Sheet1
	if isNew {
		f.DeleteSheet("Sheet1")
	}

	_, err := f.NewSheet("point")
	if err != nil {
		f.Close()
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
		f.SetCellValue("point", cellName("A", row), p.Name)
		f.SetCellValue("point", cellName("B", row), int(p.IOA))
		f.SetCellValue("point", cellName("C", row), string(p.ValueType))
		f.SetCellValue("point", cellName("D", row), string(p.PointType))
		f.SetCellValue("point", cellName("E", row), p.Efficient)
		f.SetCellValue("point", cellName("F", row), p.BaseValue)
		f.SetCellValue("point", cellName("G", row), p.Alias)

		// Modbus 扩展列
		if p.FunctionCode > 0 {
			f.SetCellValue("point", cellName("H", row), int(p.RegisterAddress))
			f.SetCellValue("point", cellName("I", row), int(p.FunctionCode))
		}
	}

	if err := f.SaveAs(path); err != nil {
		f.Close()
		return fmt.Errorf("save xlsx: %w", err)
	}
	f.Close()
	return nil
}

// TempSaveToXLSX 先写到临时文件再 rename，保证原子性
func TempSaveToXLSX(points []*Point, path string) error {
	dir := filepath.Dir(path)
	tmpPath := filepath.Join(dir, ".tmp_"+filepath.Base(path))

	if err := SaveToXLSX(points, tmpPath); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// cellName 将列字母和行号转为 excelize 单元格坐标（如 "A1"）
func cellName(col string, row int) string {
	idx, _ := excelize.CoordinatesToCellName(colIndex(col), row)
	return idx
}

func colIndex(col string) int {
	idx := 0
	for _, c := range col {
		idx = idx*26 + int(c-'A') + 1
	}
	return idx
}
