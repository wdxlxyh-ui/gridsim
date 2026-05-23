package microgrid

import (
	"fmt"
	"strings"
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func makeTestPoint(ioa uint32, name string, value float64) *config.Point {
	return &config.Point{IOA: ioa, Name: name, Value: value, PointType: config.TypeAI}
}

func newStore(points ...*config.Point) *library.Store {
	return library.NewStore(points)
}

func TestExpressionParser(t *testing.T) {
	tests := []struct {
		expr   string
		expect float64
		fail   bool
	}{
		{"2 + 3", 5, false},
		{"4 * 5", 20, false},
		{"2 + 3 * 4", 14, false},
		{"(2 + 3) * 4", 20, false},
		{"((1+2)*(3+4))", 21, false},
		{"10 - 3 - 2", 5, false},
		{"100 / 4", 25, false},
		{"-5 + 10", 5, false},
		{"3.5 * 2", 7, false},
		{"-(3+2)", -5, false},
		{"(10 + 2) * (8 - 3) / 2", 30, false},
		{"", 0, true},
		{"5 / 0", 0, true},
		{"(1 + 2", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			v, err := evaluateExpr(tt.expr)
			if tt.fail {
				if err == nil {
					t.Errorf("expected error, got %f", v)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if v != tt.expect {
				t.Errorf("got %f, want %f", v, tt.expect)
			}
		})
	}
}

func TestFormulaRefRE(t *testing.T) {
	expr := "{PV1_Power} + {Load1_Power} - {Battery1_Power}"
	matches := formulaRefRE.FindAllStringSubmatch(expr, -1)
	if len(matches) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(matches))
	}
	if matches[0][1] != "PV1_Power" || matches[1][1] != "Load1_Power" || matches[2][1] != "Battery1_Power" {
		t.Errorf("wrong refs: %v", matches)
	}
}

func resolveAndEval(store *library.Store, expr string) (float64, error) {
	res := formulaRefRE.ReplaceAllStringFunc(expr, func(m string) string {
		name := m[1 : len(m)-1]
		for _, p := range store.GetAll() {
			if p.Name == name {
				return fmt.Sprintf("%f", p.Value)
			}
		}
		return "0"
	})
	res = strings.ReplaceAll(res, " ", "")
	return evaluateExpr(res)
}

func TestResolveWithStore(t *testing.T) {
	s := newStore(
		makeTestPoint(10001, "PV1_Power", 45),
		makeTestPoint(10002, "Load1_Power", 30),
		makeTestPoint(10003, "Battery1_Power", -10),
	)

	t.Run("sum", func(t *testing.T) {
		v, _ := resolveAndEval(s, "{PV1_Power} + {Load1_Power}")
		if v != 75 {
			t.Errorf("got %f, want 75", v)
		}
	})
	t.Run("net_power", func(t *testing.T) {
		v, _ := resolveAndEval(s, "{PV1_Power} + {Battery1_Power} - {Load1_Power}")
		if v != 5 {
			t.Errorf("got %f, want 5", v)
		}
	})
	t.Run("user_scenario", func(t *testing.T) {
		v, _ := resolveAndEval(s, "{Battery1_Power} + {Load1_Power}")
		if v != 20 {
			t.Errorf("got %f, want 20", v)
		}
	})
}

func TestFormulaEvalOnStore(t *testing.T) {
	s := newStore(
		makeTestPoint(10001, "PV1_Power", 50),
		makeTestPoint(10002, "PV2_Power", 30),
		makeTestPoint(10003, "Load1_Power", 40),
		makeTestPoint(10004, "GRID_P", 0),
		makeTestPoint(10005, "PV_Total", 0),
	)

	formulas := []FormulaRule{
		{Name: "pv_sum", Target: "PV_Total", Expression: "{PV1_Power} + {PV2_Power}", Enabled: true},
		{Name: "grid", Target: "GRID_P", Expression: "{PV1_Power}+{PV2_Power}-{Load1_Power}", Enabled: true},
	}

	for _, f := range formulas {
		val, err := resolveAndEval(s, f.Expression)
		if err != nil {
			t.Fatalf("formula %s failed: %v", f.Name, err)
		}
		for _, p := range s.GetAll() {
			if p.Name == f.Target {
				s.SetValue(p.IOA, val)
				break
			}
		}
	}

	for _, p := range s.GetAll() {
		if p.Name == "PV_Total" && p.Value != 80 {
			t.Errorf("PV_Total = %f, want 80", p.Value)
		}
		if p.Name == "GRID_P" && p.Value != 40 {
			t.Errorf("GRID_P = %f, want 40", p.Value)
		}
	}
}
