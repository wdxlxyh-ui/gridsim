package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type CollectionItem struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Method    string            `json:"method,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	PreScript string            `json:"pre_script,omitempty"`
	Children  []*CollectionItem `json:"children,omitempty"`
}

type Environment struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
}

type ProxyStore struct {
	mu           sync.RWMutex
	filePath     string
	Collections  []*CollectionItem `json:"collections"`
	Environments []*Environment    `json:"environments"`
	ActiveEnvID  string            `json:"active_env_id"`
}

func NewProxyStore(configDir string) *ProxyStore {
	return &ProxyStore{
		filePath:     filepath.Join(configDir, "proxy-store.json"),
		Collections:  []*CollectionItem{},
		Environments: []*Environment{},
	}
}

func (s *ProxyStore) Load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.Save()
		}
		return err
	}
	return json.Unmarshal(data, s)
}

func (s *ProxyStore) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *ProxyStore) GetCollections() []*CollectionItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Collections
}

func (s *ProxyStore) SaveCollection(item *CollectionItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.Collections {
		if c.ID == item.ID {
			s.Collections[i] = item
			return s.Save()
		}
	}
	s.Collections = append(s.Collections, item)
	return s.Save()
}

func (s *ProxyStore) DeleteCollection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Collections = deleteByID(s.Collections, id)
	return s.Save()
}

func deleteByID(items []*CollectionItem, id string) []*CollectionItem {
	result := make([]*CollectionItem, 0, len(items))
	for _, item := range items {
		if item.ID == id {
			continue
		}
		if item.Children != nil {
			item.Children = deleteByID(item.Children, id)
		}
		result = append(result, item)
	}
	return result
}

func (s *ProxyStore) GetEnvironments() []*Environment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Environments
}

func (s *ProxyStore) GetActiveEnv() *Environment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, env := range s.Environments {
		if env.ID == s.ActiveEnvID {
			return env
		}
	}
	return nil
}

func (s *ProxyStore) SaveEnvironment(env *Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.Environments {
		if e.ID == env.ID {
			s.Environments[i] = env
			return s.Save()
		}
	}
	s.Environments = append(s.Environments, env)
	return s.Save()
}

func (s *ProxyStore) DeleteEnvironment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Environment, 0, len(s.Environments))
	for _, env := range s.Environments {
		if env.ID != id {
			result = append(result, env)
		}
	}
	s.Environments = result
	if s.ActiveEnvID == id {
		s.ActiveEnvID = ""
	}
	return s.Save()
}

func (s *ProxyStore) SetActiveEnv(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveEnvID = id
	return s.Save()
}
