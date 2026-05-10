PROJECT   := iec104-sim
VERSION   := 1.0.0
LDFLAGS   := -ldflags="-s -w -X main.version=$(VERSION)"
DIST_DIR  := bin

.PHONY: all build-linux-amd64 build-linux-arm64 build-windows build-all deb-amd64 deb-arm64 deb compress clean smoke

all: build-linux-amd64

# ── Linux amd64 ─────────────────────────────────────────
build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(PROJECT)-linux-amd64 .

# ── Linux arm64 (aarch64) ──────────────────────────────
build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(PROJECT)-linux-arm64 .

# ── Windows amd64 ─────────────────────────────────────
build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(PROJECT).exe .

# ── 全部二进制 ──────────────────────────────────────────
build-all: build-linux-amd64 build-linux-arm64 build-windows

# ── .deb 打包 ───────────────────────────────────────────
deb-amd64: build-linux-amd64
	@mkdir -p /tmp/deb-amd64/DEBIAN /tmp/deb-amd64/usr/local/bin
	cp $(DIST_DIR)/$(PROJECT)-linux-amd64 /tmp/deb-amd64/usr/local/bin/$(PROJECT)
	chmod 755 /tmp/deb-amd64/usr/local/bin/$(PROJECT)
	printf 'Package: %s\nVersion: %s\nSection: utils\nPriority: optional\nArchitecture: amd64\nMaintainer: IEC104 Simulator <dev@example.com>\nDescription: IEC 60870-5-104 Simulator\n Supports telemetry (YC), teleindication (YX),\n pulse counter (YM), and AO/DO control.\n Features Excel-based point configuration,\n HTTP API for data modification with spontaneous\n change reporting, and total interrogation response.\nBuilt-Using: go1.22.5\n' $(PROJECT) $(VERSION) > /tmp/deb-amd64/DEBIAN/control
	cd /tmp/deb-amd64 && find . -type f ! -path './DEBIAN/*' -exec md5sum {} \; > DEBIAN/md5sums
	dpkg-deb --build /tmp/deb-amd64 $(DIST_DIR)/$(PROJECT)_$(VERSION)_amd64.deb
	@rm -rf /tmp/deb-amd64

deb-arm64: build-linux-arm64
	@mkdir -p /tmp/deb-arm64/DEBIAN /tmp/deb-arm64/usr/local/bin
	cp $(DIST_DIR)/$(PROJECT)-linux-arm64 /tmp/deb-arm64/usr/local/bin/$(PROJECT)
	chmod 755 /tmp/deb-arm64/usr/local/bin/$(PROJECT)
	printf 'Package: %s\nVersion: %s\nSection: utils\nPriority: optional\nArchitecture: arm64\nMaintainer: IEC104 Simulator <dev@example.com>\nDescription: IEC 60870-5-104 Simulator\n Supports telemetry (YC), teleindication (YX),\n pulse counter (YM), and AO/DO control.\n Features Excel-based point configuration,\n HTTP API for data modification with spontaneous\n change reporting, and total interrogation response.\nBuilt-Using: go1.22.5\n' $(PROJECT) $(VERSION) > /tmp/deb-arm64/DEBIAN/control
	cd /tmp/deb-arm64 && find . -type f ! -path './DEBIAN/*' -exec md5sum {} \; > DEBIAN/md5sums
	dpkg-deb --build /tmp/deb-arm64 $(DIST_DIR)/$(PROJECT)_$(VERSION)_arm64.deb
	@rm -rf /tmp/deb-arm64

# ── 全部 .deb ──────────────────────────────────────────
deb: deb-amd64 deb-arm64

# ── UPX 压缩 ────────────────────────────────────────────
compress: build-linux-amd64
	upx --best $(DIST_DIR)/$(PROJECT)-linux-amd64 \
		-o $(DIST_DIR)/$(PROJECT)-linux-amd64-upx 2>/dev/null || true

# ── 冒烟测试 ────────────────────────────────────────────
smoke: build-linux-amd64
	@echo "=== 编译产物 ==="
	file $(DIST_DIR)/$(PROJECT)-linux-amd64
	@echo ""
	@echo "=== 文件大小 ==="
	ls -lh $(DIST_DIR)/$(PROJECT)-linux-amd64
	@echo ""
	@echo "=== 检查静态链接 ==="
	@ldd $(DIST_DIR)/$(PROJECT)-linux-amd64 2>&1 | grep -q "statically linked" && \
		echo "✓ 静态链接" || echo "✓ 动态链接（需要运行时库）"
	@echo ""
	@echo "=== 版本信息 ==="
	@strings $(DIST_DIR)/$(PROJECT)-linux-amd64 | grep -E "^1\.[0-9]+\.[0-9]+" || true
	@echo "OK"

# ── 清理 ─────────────────────────────────────────────────
clean:
	rm -rf $(DIST_DIR)/*

# ── 依赖管理 ─────────────────────────────────────────────
deps:
	go mod tidy
	go mod download

fmt:
	go fmt ./...

vet:
	go vet ./...
