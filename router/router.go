package router

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// run executes a shell command and returns combined stdout+stderr with ANSI codes stripped.
func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(ansiEscape.ReplaceAllString(buf.String(), ""))
	return out, err
}

// Reboot schedules a router reboot (returns before the router goes down).
func Reboot() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := run(ctx, "reboot")
	return err
}

// XkeenCmd runs /opt/sbin/xkeen with the given action (start/stop/restart/status).
// xkeen expects flags with a leading dash, e.g. -restart.
func XkeenCmd(xkeenPath, action string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := run(ctx, xkeenPath, "-"+action)
	if err != nil {
		return out, fmt.Errorf("xkeen %s: %w", action, err)
	}
	return out, nil
}
