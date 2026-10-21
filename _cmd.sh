set -e
export PATH=/usr/local/go/bin:/home/egc/IEC-SIM/toolchain/node-v24.16.0/bin:$PATH
B=/home/egc/IEC-SIM/tsdb-build
rm -rf $B && mkdir -p $B && cd $B
tar xzf /home/egc/IEC-SIM/gridsim_src.tar.gz
go version; node -v
if [ -d /home/egc/IEC-SIM/iec104-sim-master/web/node_modules ]; then cp -r /home/egc/IEC-SIM/iec104-sim-master/web/node_modules web/node_modules; fi
export TMPDIR=$B/.gotmp; mkdir -p $TMPDIR
echo "== go vet"; go vet ./cmd/gridsim/ 2>&1 | tail -30
echo "== go build"; go build -o $B/gridsim-bin ./cmd/gridsim/ 2>&1 | tail -30; ls -lh $B/gridsim-bin
echo "== web (vue-tsc + vite)"; cd web && npm install --silent 2>&1 | tail -5; npm run build 2>&1 | grep -Ev "compressed file|\.gz " | tail -25
echo "BUILD_DONE"
