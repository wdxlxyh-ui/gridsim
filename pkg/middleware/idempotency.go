package middleware

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

type cachedResponse struct {
	statusCode int
	header     http.Header
	body       []byte
	expiresAt  time.Time
}

var (
	idemCache   sync.Map
	idemTTL     = 24 * time.Hour
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func IdempotencyMiddleware(next http.Handler) http.Handler {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			now := time.Now()
			idemCache.Range(func(key, val any) bool {
				if cr, ok := val.(*cachedResponse); ok && now.After(cr.expiresAt) {
					idemCache.Delete(key)
				}
				return true
			})
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" || (r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch && r.Method != http.MethodDelete) {
			next.ServeHTTP(w, r)
			return
		}

		cacheKey := r.Method + ":" + r.URL.Path + ":" + key

		if val, ok := idemCache.Load(cacheKey); ok {
			if cr, ok := val.(*cachedResponse); ok && time.Now().Before(cr.expiresAt) {
				for k, v := range cr.header {
					w.Header()[k] = v
				}
				w.WriteHeader(cr.statusCode)
				w.Write(cr.body)
				return
			}
			idemCache.Delete(cacheKey)
		}

		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		if rec.statusCode < 500 {
			idemCache.Store(cacheKey, &cachedResponse{
				statusCode: rec.statusCode,
				header:     rec.Header().Clone(),
				body:       rec.body.Bytes(),
				expiresAt:  time.Now().Add(idemTTL),
			})
		}
	})
}
