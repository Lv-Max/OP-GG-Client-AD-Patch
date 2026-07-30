package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Launches the unmodified OP.GG client with the Node inspector enabled, injects
// a hook into its main process over the DevTools protocol, then detaches.
// Nothing on disk is changed, so it keeps working across OP.GG updates.

func main() {
	fmt.Println("OP.GG Launcher - https://github.com/Lv-Max/OP-GG-Client-AD-Patch")

	exe, err := findClient()
	if err != nil {
		fatal(err.Error())
	}

	// The launcher must own the first instance for --inspect to take effect
	// (the client uses a single-instance lock). Close any running copy first.
	// Tests can opt out so an isolated --user-data-dir never disturbs another
	// OP.GG instance on the same machine.
	if os.Getenv("OPGG_LAUNCHER_SKIP_KILL") != "1" {
		if killRunning() {
			time.Sleep(2 * time.Second)
		}
	}

	port := randomPort()
	fmt.Println("opening OP.GG...")
	if err := spawn(exe, port); err != nil {
		fatal("failed to start client: " + err.Error())
	}

	if err := inject(port); err != nil {
		// Never leave the user worse off: the client is already running, just
		// unpatched. Report and exit success.
		fmt.Println("patch not applied:", err.Error())
		fmt.Println("the client is running normally (no changes made).")
		holdOpen()
		return
	}
	fmt.Println("patch applied")
	holdOpen()
}

// holdOpen keeps the console visible for a few seconds so a double-click user
// can read the result before the window closes.
func holdOpen() {
	for i := 5; i > 0; i-- {
		fmt.Printf("\rclosing in %d... ", i)
		time.Sleep(time.Second)
	}
	fmt.Println()
}

func inject(port int) error {
	wsURL, err := inspectorWebSocketURL(port, 20*time.Second)
	if err != nil {
		return err
	}
	c, err := dialCDP(wsURL)
	if err != nil {
		return err
	}
	defer c.close()

	if _, err := c.send("Runtime.enable", nil, 5*time.Second); err != nil {
		return err
	}

	var last string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		last, err = c.evaluate(installJS, 5*time.Second)
		if err != nil {
			return err
		}
		if strings.HasPrefix(last, "applied:") {
			// Wait briefly for at least one renderer push when a window already
			// exists. If the window is created later, the installed listener
			// will handle it after the inspector has closed.
			pushDeadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(pushDeadline) {
				status, statusErr := c.evaluate(patchStatusJS, 3*time.Second)
				if statusErr != nil || strings.Contains(status, ":push=") && !strings.HasSuffix(status, "pending") {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			// Isolated end-to-end diagnostics may reconnect to capture a hidden
			// BrowserWindow. Production runs always close the inspector.
			if os.Getenv("OPGG_LAUNCHER_TEST_KEEP_INSPECTOR") != "1" {
				c.evaluate(closeInspectorJS, 3*time.Second)
			}
			return nil
		}
		if strings.HasPrefix(last, "error:") {
			return fmt.Errorf("hook failed: %s", last)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("hook did not take (last: %q)", last)
}

// --- environment helpers ---------------------------------------------------

func findClient() (string, error) {
	if len(os.Args) > 1 {
		if fileExists(os.Args[1]) {
			return os.Args[1], nil
		}
		return "", fmt.Errorf("given path does not exist: %s", os.Args[1])
	}
	candidates := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "OP.GG", "OP.GG.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "OP.GG", "OP.GG.exe"),
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("OP.GG.exe not found; pass its path as the first argument")
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func killRunning() bool {
	out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq OP.GG.exe", "/NH").Output()
	if !strings.Contains(string(out), "OP.GG.exe") {
		return false
	}
	exec.Command("taskkill", "/IM", "OP.GG.exe", "/F").Run()
	return true
}

func spawn(exe string, port int) error {
	args := []string{fmt.Sprintf("--inspect=127.0.0.1:%d", port)}
	if userDataDir := os.Getenv("OPGG_LAUNCHER_USER_DATA_DIR"); userDataDir != "" {
		args = append(args, "--user-data-dir="+userDataDir)
	}
	if debugPort := os.Getenv("OPGG_LAUNCHER_REMOTE_DEBUGGING_PORT"); debugPort != "" {
		args = append(args, "--remote-debugging-port="+debugPort)
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	if os.Getenv("OPGG_LAUNCHER_INHERIT_STDIO") == "1" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Detach: let the client keep running after the launcher exits.
	go func() { cmd.Process.Release() }()
	return nil
}

func randomPort() int {
	n, err := rand.Int(rand.Reader, big.NewInt(20000))
	if err != nil {
		return 39217
	}
	return 30000 + int(n.Int64())
}

func fatal(msg string) {
	fmt.Println("error:", msg)
	fmt.Println()
	fmt.Println("press Enter to close...")
	fmt.Scanln()
	os.Exit(1)
}
