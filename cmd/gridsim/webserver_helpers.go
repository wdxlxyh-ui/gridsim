package main

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

// 文件系统辅助方法

func (ws *webServer) readFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

func (ws *webServer) writeFile(path string, data []byte) error {
	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0644)
}

func (ws *webServer) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (ws *webServer) removeFile(path string) error {
	return os.Remove(path)
}

func (ws *webServer) readDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (ws *webServer) ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}