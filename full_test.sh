#!/bin/bash
export PATH="$HOME/go-toolchain/go/bin:$PATH"

cd /mnt/d/AI/Claw/gridsim

echo "========================================"
echo " Phase 1: go vet"
echo "========================================"
go vet ./... 2>&1
echo "Exit: $?"

echo ""
echo "========================================"
echo " Phase 2: Unit Tests (all packages)"
echo "========================================"
go test -v -count=1 -timeout 60s ./... 2>&1
UT_EXIT=$?
echo "Unit Tests Exit: $UT_EXIT"

echo ""
echo "========================================"
echo " Phase 3: Race Detection"
echo "========================================"
go test -race -count=1 -timeout 60s ./... 2>&1
RD_EXIT=$?
echo "Race Detection Exit: $RD_EXIT"

echo ""
echo "========================================"
echo " Phase 4: Build Check"
echo "========================================"
go build -ldflags="-s -w" -o /tmp/gridsim-test . 2>&1
echo "Build Exit: $?"
ls -la /tmp/gridsim-test
file /tmp/gridsim-test
rm /tmp/gridsim-test

echo ""
echo "========================================"
echo " Phase 5: Integration Test (HTTP API)"
echo "========================================"
bash /mnt/d/AI/Claw/gridsim/integration_test.sh 2>&1
INT_EXIT=$?

echo ""
echo "========================================"
echo " SUMMARY"
echo "========================================"
echo "Unit Tests:     $([ $UT_EXIT -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Race Detection: $([ $RD_EXIT -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Integration:    $([ $INT_EXIT -eq 0 ] && echo 'PASS' || echo 'FAIL')"

exit $((UT_EXIT + RD_EXIT + INT_EXIT))
