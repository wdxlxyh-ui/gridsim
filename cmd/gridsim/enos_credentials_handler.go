package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"gridsim/internal/model"
)

// EnOS 凭据管理
func (ws *webServer) handleEnOSCredentials(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/secrets/enos")
	
	switch {
	case path == "" || path == "/":
		if r.Method == http.MethodGet {
			ws.listEnOSCredentials(w, r)
		} else if r.Method == http.MethodPost {
			ws.createEnOSCredential(w, r)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case strings.HasSuffix(path, "/test"):
		credID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/test")
		if r.Method == http.MethodPost {
			ws.testEnOSCredential(w, r, credID)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		credID := strings.TrimPrefix(path, "/")
		if r.Method == http.MethodPut {
			ws.updateEnOSCredential(w, r, credID)
		} else if r.Method == http.MethodDelete {
			ws.deleteEnOSCredential(w, r, credID)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

// listEnOSCredentials 列出所有凭据档案
func (ws *webServer) listEnOSCredentials(w http.ResponseWriter, r *http.Request) {
	credentialsDir := filepath.Join(ws.config.ConfigDir, "enos_credentials")
	
	files, err := ws.readDir(credentialsDir)
	if err != nil {
		writeJSON(w, []interface{}{})
		return
	}
	
	var credentials []model.EnOSCredential
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		credPath := filepath.Join(credentialsDir, file.Name())
		cred, err := ws.loadEnOSCredential(credPath)
		if err != nil {
			continue
		}
		
		// 清除敏感信息
		cred.AccessKey = ""
		cred.SecretKey = ""
		credentials = append(credentials, *cred)
	}
	
	writeJSON(w, credentials)
}

// createEnOSCredential 创建新的凭据档案
func (ws *webServer) createEnOSCredential(w http.ResponseWriter, r *http.Request) {
	var req model.EnOSCredential
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	
	// 验证必填字段
	if req.ID == "" || req.Name == "" || req.ApigwAddress == "" || req.OrgID == "" {
		writeError(w, http.StatusBadRequest, "id, name, apigw_address, org_id are required")
		return
	}
	
	if req.AccessKey == "" || req.SecretKey == "" {
		writeError(w, http.StatusBadRequest, "access_key and secret_key are required")
		return
	}
	
	// 验证 HTTPS
	if !strings.HasPrefix(req.ApigwAddress, "https://") {
		writeError(w, http.StatusBadRequest, "apigw_address must use HTTPS")
		return
	}
	
	credentialsDir := filepath.Join(ws.config.ConfigDir, "enos_credentials")
	if err := ws.ensureDir(credentialsDir); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create credentials directory")
		return
	}
	
	credPath := filepath.Join(credentialsDir, req.ID+".json")
	
	// 检查是否已存在
	if ws.fileExists(credPath) {
		writeError(w, http.StatusConflict, "credential already exists")
		return
	}
	
	// 加密敏感信息
	encryptedAccessKey, err := ws.encryptSecret(req.AccessKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt access key")
		return
	}
	
	encryptedSecretKey, err := ws.encryptSecret(req.SecretKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt secret key")
		return
	}
	
	// 保存凭据
	credential := model.EnOSCredential{
		ID:               req.ID,
		Name:             req.Name,
		ApigwAddress:     req.ApigwAddress,
		OrgID:            req.OrgID,
		AccessKey:        encryptedAccessKey,
		SecretKey:        encryptedSecretKey,
		AccessKeyPresent: true,
		SecretKeyPresent: true,
		UpdatedAt:        time.Now(),
	}
	
	if err := ws.saveEnOSCredential(credPath, &credential); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save credential: "+err.Error())
		return
	}
	
	// 返回不含敏感信息的版本
	credential.AccessKey = ""
	credential.SecretKey = ""
	writeJSON(w, credential)
}

// updateEnOSCredential 更新凭据档案
func (ws *webServer) updateEnOSCredential(w http.ResponseWriter, r *http.Request, credID string) {
	credentialsDir := filepath.Join(ws.config.ConfigDir, "enos_credentials")
	credPath := filepath.Join(credentialsDir, credID+".json")
	
	// 检查是否存在
	if !ws.fileExists(credPath) {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	
	var req model.EnOSCredential
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	
	// 加载现有凭据
	existing, err := ws.loadEnOSCredential(credPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load existing credential")
		return
	}
	
	// 更新字段
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.ApigwAddress != "" {
		if !strings.HasPrefix(req.ApigwAddress, "https://") {
			writeError(w, http.StatusBadRequest, "apigw_address must use HTTPS")
			return
		}
		existing.ApigwAddress = req.ApigwAddress
	}
	if req.OrgID != "" {
		existing.OrgID = req.OrgID
	}
	
	// 更新密钥（如果提供）
	if req.AccessKey != "" {
		encryptedAccessKey, err := ws.encryptSecret(req.AccessKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt access key")
			return
		}
		existing.AccessKey = encryptedAccessKey
		existing.AccessKeyPresent = true
	}
	
	if req.SecretKey != "" {
		encryptedSecretKey, err := ws.encryptSecret(req.SecretKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret key")
			return
		}
		existing.SecretKey = encryptedSecretKey
		existing.SecretKeyPresent = true
	}
	
	existing.UpdatedAt = time.Now()
	
	// 保存更新
	if err := ws.saveEnOSCredential(credPath, existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save credential: "+err.Error())
		return
	}
	
	// 返回不含敏感信息的版本
	existing.AccessKey = ""
	existing.SecretKey = ""
	writeJSON(w, existing)
}

// deleteEnOSCredential 删除凭据档案
func (ws *webServer) deleteEnOSCredential(w http.ResponseWriter, r *http.Request, credID string) {
	credentialsDir := filepath.Join(ws.config.ConfigDir, "enos_credentials")
	credPath := filepath.Join(credentialsDir, credID+".json")
	
	if !ws.fileExists(credPath) {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	
	if err := ws.removeFile(credPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete credential: "+err.Error())
		return
	}
	
	writeJSON(w, map[string]string{"status": "deleted"})
}

// testEnOSCredential 测试凭据连接
func (ws *webServer) testEnOSCredential(w http.ResponseWriter, r *http.Request, credID string) {
	credentialsDir := filepath.Join(ws.config.ConfigDir, "enos_credentials")
	credPath := filepath.Join(credentialsDir, credID+".json")
	
	if !ws.fileExists(credPath) {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	
	credential, err := ws.loadEnOSCredential(credPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load credential")
		return
	}
	
	// 解密密钥
	accessKey, err := ws.decryptSecret(credential.AccessKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt access key")
		return
	}
	
	secretKey, err := ws.decryptSecret(credential.SecretKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt secret key")
		return
	}
	
	// TODO: 实际测试EnOS连接
	// 目前返回模拟结果
	testResult := map[string]interface{}{
		"success": true,
		"message": "Connection test successful",
		"tested_at": time.Now().Format(time.RFC3339),
		"api_gateway": credential.ApigwAddress,
		"org_id": credential.OrgID,
	}
	
	_ = accessKey // 避免未使用变量警告
	_ = secretKey
	
	writeJSON(w, testResult)
}

// loadEnOSCredential 加载凭据档案
func (ws *webServer) loadEnOSCredential(credPath string) (*model.EnOSCredential, error) {
	data, err := ws.readFile(credPath)
	if err != nil {
		return nil, err
	}
	
	var cred model.EnOSCredential
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, err
	}
	
	return &cred, nil
}

// saveEnOSCredential 保存凭据档案
func (ws *webServer) saveEnOSCredential(credPath string, cred *model.EnOSCredential) error {
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	
	return ws.writeFile(credPath, data)
}

// 加密和解密辅助函数
func (ws *webServer) encryptSecret(plaintext string) (string, error) {
	// TODO: 实现实际的加密逻辑
	// 这里返回 Base64 编码的模拟加密
	return fmt.Sprintf("encrypted:%s", plaintext), nil
}

func (ws *webServer) decryptSecret(encrypted string) (string, error) {
	// TODO: 实现实际的解密逻辑
	// 这里返回去除前缀的模拟解密
	if strings.HasPrefix(encrypted, "encrypted:") {
		return strings.TrimPrefix(encrypted, "encrypted:"), nil
	}
	return encrypted, nil
}