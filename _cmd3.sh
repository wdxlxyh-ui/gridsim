export PATH=/usr/local/go/bin:$PATH
B=/home/egc/IEC-SIM/tsdb-build
cd $B
ls -lh gridsim-bin
echo "== vet"; export TMPDIR=$B/.gotmp; go vet ./cmd/gridsim/ 2>&1 | tail -10; echo "vet rc=$?"
echo "== dist"; ls web/dist | head; grep -l "tsdb-config" web/dist/assets/js/PyMicrogridEditor-*.js 2>/dev/null
ls cmd/gridsim/resources 2>/dev/null | head -3
ls config | head
