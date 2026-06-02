package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ProxyRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Timeout int               `json:"timeout"`
}

type ProxyResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"status_text"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	TimeMs     int64             `json:"time_ms"`
	Size       int               `json:"size"`
	Error      string            `json:"error,omitempty"`
}

type ProxyHandler struct{}

func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{}
}

func (h *ProxyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/proxy", h.handleProxy)
}

func (h *ProxyHandler) handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	timeout := 30
	if req.Timeout > 0 && req.Timeout <= 120 {
		timeout = req.Timeout
	}

	result := h.executeRequest(req, time.Duration(timeout)*time.Second)
	writeJSON(w, http.StatusOK, result)
}

func (h *ProxyHandler) executeRequest(req ProxyRequest, timeout time.Duration) ProxyResponse {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = bytes.NewReader([]byte(req.Body))
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return ProxyResponse{Error: "invalid request: " + err.Error()}
	}

	for k, v := range req.Headers {
		if strings.EqualFold(k, "host") {
			continue
		}
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	elapsed := time.Since(start)

	if err != nil {
		return ProxyResponse{
			Error:  err.Error(),
			TimeMs: elapsed.Milliseconds(),
		}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))

	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	statusText := http.StatusText(resp.StatusCode)
	return ProxyResponse{
		Status:     resp.StatusCode,
		StatusText: statusText,
		Headers:    respHeaders,
		Body:       string(bodyBytes),
		TimeMs:     elapsed.Milliseconds(),
		Size:       len(bodyBytes),
	}
}

var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

func ReplaceVars(text string, vars map[string]string) string {
	if len(vars) == 0 {
		return text
	}
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		key := match[2 : len(match)-2]
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}
