// Package firewall provides iptables rule management for auto-opening ports.
package firewall

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// EnsurePort checks if an iptables ACCEPT rule exists for the given port,
// and adds one if missing. Returns true if a rule was added.
// Errors are logged but not returned — failures are non-fatal.
func EnsurePort(port int, comment string) (added bool) {
	exists, err := ruleExists(port)
	if err != nil {
		slog.Warn("防火墙: 检查端口失败，跳过自动配置", "port", port, "error", err)
		return false
	}
	if exists {
		return false
	}

	if err := addRule(port, comment); err != nil {
		slog.Warn("防火墙: 添加规则失败", "port", port, "error", err)
		return false
	}

	slog.Info("防火墙: 已放行端口", "port", port, "comment", comment)

	if err := saveRules(); err != nil {
		slog.Debug("防火墙: 持久化保存失败（非致命）", "error", err)
	}

	return true
}

// RemovePort removes the iptables rule for the given port.
func RemovePort(port int) {
	lines, err := findRuleLines(port)
	if err != nil || len(lines) == 0 {
		return
	}

	for i := len(lines) - 1; i >= 0; i-- {
		cmd := exec.Command("iptables", "-D", "INPUT", fmt.Sprintf("%d", lines[i]))
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Warn("防火墙: 删除规则失败", "port", port, "line", lines[i], "error", string(out))
		}
	}

	slog.Info("防火墙: 已移除端口规则", "port", port)
	saveRules()
}

func ruleExists(port int) (bool, error) {
	cmd := exec.Command("iptables", "-L", "INPUT", "-n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("iptables -L failed: %w", err)
	}
	target := fmt.Sprintf("dpt:%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, target) && strings.Contains(line, "ACCEPT") {
			return true, nil
		}
	}
	return false, nil
}

func findRuleLines(port int) ([]int, error) {
	cmd := exec.Command("iptables", "-L", "INPUT", "-n", "--line-numbers")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iptables -L failed: %w", err)
	}
	target := fmt.Sprintf("dpt:%d", port)
	var lines []int
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, target) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				var n int
				if _, err := fmt.Sscanf(fields[0], "%d", &n); err == nil {
					lines = append(lines, n)
				}
			}
		}
	}
	return lines, nil
}

func addRule(port int, comment string) error {
	args := []string{"-I", "INPUT", "-p", "tcp", "--dport", fmt.Sprintf("%d", port)}
	if comment != "" {
		args = append(args, "-m", "comment", "--comment", comment)
	}
	args = append(args, "-j", "ACCEPT")

	cmd := exec.Command("iptables", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables add failed: %s", string(out))
	}
	return nil
}

func saveRules() error {
	cmd := exec.Command("sh", "-c", "iptables-save > /etc/iptables/rules.v4 2>/dev/null")
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("netfilter-persistent", "save")
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("sh", "-c", "iptables-save 2>/dev/null > /etc/iptables/rules.v4")
	return cmd.Run()
}
