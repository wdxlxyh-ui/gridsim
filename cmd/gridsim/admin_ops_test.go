package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gridsim/internal/model"
	"gridsim/pkg/middleware"
	"golang.org/x/crypto/bcrypt"
)

func adminRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	return req.WithContext(context.WithValue(req.Context(), middleware.UsernameKey, "admin"))
}

func TestAdminPasswordChangeRequiresCurrentPasswordAndSavesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	ws := &webServer{
		userConfigPath: path,
		userConfig: &model.UserConfig{Users: []model.User{{ID: "1", Username: "admin", Role: "admin", PasswordHash: string(oldHash)}}},
	}
	if err := saveUserConfigAtomic(path, ws.userConfig); err != nil {
		t.Fatal(err)
	}

	req := adminRequest(t, http.MethodPut, "/api/v1/admin/password", changePasswordRequest{
		CurrentPassword: "old-password",
		NewPassword:     "new-password",
		ConfirmPassword: "new-password",
	})
	resp := httptest.NewRecorder()
	ws.handleAdminPassword(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved model.UserConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(saved.Users[0].PasswordHash), []byte("new-password")) != nil {
		t.Fatal("new password was not persisted")
	}

	badReq := adminRequest(t, http.MethodPut, "/api/v1/admin/password", changePasswordRequest{
		CurrentPassword: "wrong-password",
		NewPassword:     "another-password",
		ConfirmPassword: "another-password",
	})
	badResp := httptest.NewRecorder()
	ws.handleAdminPassword(badResp, badReq)
	if badResp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d, want 401", badResp.Code)
	}
}

func TestAdminClearRejectsWrongConfirmationBeforeBackup(t *testing.T) {
	ws := &webServer{userConfig: &model.UserConfig{Users: []model.User{{Username: "admin", Role: "admin", PasswordHash: mustAdminHash(t, "password")}}}}
	req := adminRequest(t, http.MethodPost, "/api/v1/admin/database/clear", adminConfirmRequest{
		Password: "password", Confirmation: "WRONG",
	})
	resp := httptest.NewRecorder()
	ws.handleAdminDatabaseClear(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", resp.Code)
	}
}

func TestCopyAdminTreeCopiesRegularFiles(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "data.txt"), []byte("backup"), 0o640); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "backup")
	if err := copyAdminTree(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "nested", "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "backup" {
		t.Fatalf("copied data=%q", data)
	}
}

func mustAdminHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}
