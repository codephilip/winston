package kali

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Status represents the result of an SSH connectivity check to the Kali VM.
type Status struct {
	Host   string `json:"host"`
	Status string `json:"status"` // "online" or "offline"
	Info   string `json:"info"`   // output from uname -a && uptime
}

// CheckConnectivity tests SSH access to the Kali VM by running a remote command.
// It reads KALI_VM_HOST, KALI_VM_USER, and KALI_VM_SSH_KEY from environment variables.
func CheckConnectivity() (*Status, error) {
	host := os.Getenv("KALI_VM_HOST")
	user := os.Getenv("KALI_VM_USER")
	keyPath := os.Getenv("KALI_VM_SSH_KEY")

	if host == "" || user == "" {
		return &Status{
			Host:   host,
			Status: "offline",
			Info:   "KALI_VM_HOST or KALI_VM_USER not configured",
		}, fmt.Errorf("KALI_VM_HOST or KALI_VM_USER not set")
	}

	target := fmt.Sprintf("%s@%s", user, host)

	args := []string{
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
	}
	if keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	args = append(args, target, "uname -a && uptime")

	cmd := exec.Command("ssh", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &Status{
			Host:   host,
			Status: "offline",
			Info:   strings.TrimSpace(string(output)),
		}, nil
	}

	return &Status{
		Host:   host,
		Status: "online",
		Info:   strings.TrimSpace(string(output)),
	}, nil
}
