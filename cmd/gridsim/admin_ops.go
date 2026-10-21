package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gridsim/internal/model"
	"gridsim/pkg/middleware"
	"golang.org/x/crypto/bcrypt"
)

const (
	clearDatabaseConfirmation = "CLEAR_DATABASE"
	clearConfigConfirmation   = "DELETE_ALL_CONFIG"
	minPasswordRunes          = 8
	maxPasswordRunes          = 128
)

type adminConfirmRequest struct {
	Password     string `json:"password"`
	Confirmation string `json:"confirmation"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (ws *webServer) handleAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/admin/password":
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ws.handleAdminPassword(w, r)
	case "/api/v1/admin/database/clear":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ws.handleAdminDatabaseClear(w, r)
	case "/api/v1/admin/config/clear":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ws.handleAdminConfigClear(w, r)
	default:
		writeError(w, http.StatusNotFound, "admin operation not found")
	}
}

func (ws *webServer) adminUsername(r *http.Request) (string, bool) {
	username, ok := r.Context().Value(middleware.UsernameKey).(string)
	return username, ok && strings.TrimSpace(username) != ""
}

func (ws *webServer) verifyAdminPassword(r *http.Request, password string) (string, error) {
	username, ok := ws.adminUsername(r)
	if !ok {
		return "", fmt.Errorf("admin username is missing from token")
	}
	ws.userMu.RLock()
	defer ws.userMu.RUnlock()
	if ws.userConfig == nil {
		return "", fmt.Errorf("user config not loaded")
	}
	for _, user := range ws.userConfig.Users {
		if user.Username != username {
			continue
		}
		if user.Role != "admin" {
			return "", fmt.Errorf("user is no longer an admin")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			return "", fmt.Errorf("current password is incorrect")
		}
		return username, nil
	}
	return "", fmt.Errorf("admin user not found")
}

func decodeAdminJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return err
	}
	return nil
}

func (ws *webServer) handleAdminPassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if decodeAdminJSON(w, r, &req) != nil {
		return
	}
	username, err := ws.verifyAdminPassword(r, req.CurrentPassword)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "new passwords do not match")
		return
	}
	runes := utf8.RuneCountInString(req.NewPassword)
	if runes < minPasswordRunes || runes > maxPasswordRunes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("new password must contain %d-%d characters", minPasswordRunes, maxPasswordRunes))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	ws.userMu.Lock()
	defer ws.userMu.Unlock()
	if ws.userConfig == nil {
		writeError(w, http.StatusInternalServerError, "user config not loaded")
		return
	}
	updated := false
	oldHash := ""
	for i := range ws.userConfig.Users {
		if ws.userConfig.Users[i].Username == username {
			oldHash = ws.userConfig.Users[i].PasswordHash
			ws.userConfig.Users[i].PasswordHash = string(hash)
			updated = true
			break
		}
	}
	if !updated {
		writeError(w, http.StatusNotFound, "admin user not found")
		return
	}
	if err := saveUserConfigAtomic(ws.userConfigPath, ws.userConfig); err != nil {
		for i := range ws.userConfig.Users {
			if ws.userConfig.Users[i].Username == username {
				ws.userConfig.Users[i].PasswordHash = oldHash
				break
			}
		}
		writeError(w, http.StatusInternalServerError, "failed to save user config")
		return
	}
	slog.Info("管理员密码已修改", "username", username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func (ws *webServer) handleAdminDatabaseClear(w http.ResponseWriter, r *http.Request) {
	var req adminConfirmRequest
	if decodeAdminJSON(w, r, &req) != nil {
		return
	}
	username, err := ws.verifyAdminPassword(r, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.Confirmation != clearDatabaseConfirmation {
		writeError(w, http.StatusBadRequest, "confirmation must equal CLEAR_DATABASE")
		return
	}
	dataStore := ws.mgr.DataStore()
	if dataStore == nil {
		writeError(w, http.StatusServiceUnavailable, "data persistence is disabled")
		return
	}
	backupDir, backupRel, err := ws.createAdminBackup("database")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin backup")
		return
	}
	backupPath := filepath.Join(backupDir, "gridsim.db")
	if err := dataStore.BackupTo(backupPath); err != nil {
		writeError(w, http.StatusInternalServerError, "database backup failed")
		return
	}
	ws.mgr.StopAll()
	deleted, err := dataStore.ClearAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database clear failed")
		return
	}
	slog.Warn("管理员已清理全部数据库历史", "username", username, "deleted_rows", deleted, "backup", backupRel)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "database_cleared",
		"deleted_rows": deleted,
		"backup_path": backupRel,
	})
}

func (ws *webServer) handleAdminConfigClear(w http.ResponseWriter, r *http.Request) {
	var req adminConfirmRequest
	if decodeAdminJSON(w, r, &req) != nil {
		return
	}
	username, err := ws.verifyAdminPassword(r, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.Confirmation != clearConfigConfirmation {
		writeError(w, http.StatusBadRequest, "confirmation must equal DELETE_ALL_CONFIG")
		return
	}
	backupDir, backupRel, err := ws.createAdminBackup("config")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin backup")
		return
	}
	if err := copyAdminTree(ws.cfgDir, filepath.Join(backupDir, "config")); err != nil {
		writeError(w, http.StatusInternalServerError, "config backup failed")
		return
	}
	if dataStore := ws.mgr.DataStore(); dataStore != nil {
		dbBackupPath := filepath.Join(backupDir, "config", "db", "gridsim.db")
		if err := dataStore.BackupTo(dbBackupPath); err != nil {
			writeError(w, http.StatusInternalServerError, "database backup failed")
			return
		}
	}
	if err := ws.mgr.ClearConfigs(); err != nil {
		writeError(w, http.StatusInternalServerError, "clear instance configs failed")
		return
	}
	if err := ws.trendStore.Clear(); err != nil {
		writeError(w, http.StatusInternalServerError, "clear trend templates failed")
		return
	}
	if err := ws.proxyStore.Reset(); err != nil {
		writeError(w, http.StatusInternalServerError, "clear proxy config failed")
		return
	}
	removed, err := ws.removeBusinessConfigFiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remove business config files failed")
		return
	}
	slog.Warn("管理员已删除全部业务配置", "username", username, "removed_files", removed, "backup", backupRel)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "config_cleared",
		"removed_files": removed,
		"backup_path":   backupRel,
		"preserved":     []string{"users.json", "db", "backups", "py_simulator"},
	})
}

func (ws *webServer) createAdminBackup(kind string) (string, string, error) {
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + kind
	dir := filepath.Join(filepath.Dir(ws.cfgDir), "backups", "admin-ops", name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", "", err
	}
	return dir, filepath.ToSlash(filepath.Join("backups", "admin-ops", name)), nil
}

func (ws *webServer) removeBusinessConfigFiles() (int, error) {
	removed := 0
	for _, name := range []string{"auto_changes", "recordings", "csv", "point-table-backups"} {
		path := filepath.Join(ws.cfgDir, name)
		if _, err := os.Stat(path); err == nil {
			if err := os.RemoveAll(path); err != nil {
				return removed, err
			}
			removed++
		}
	}
	entries, err := os.ReadDir(ws.cfgDir)
	if err != nil {
		return removed, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".xlsx" && ext != ".xls" {
			continue
		}
		if err := os.Remove(filepath.Join(ws.cfgDir, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func saveUserConfigAtomic(path string, cfg *model.UserConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".users-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(tmpPath, path)
	}
	return nil
}

func copyAdminTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyAdminTree(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := copyAdminFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyAdminFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
