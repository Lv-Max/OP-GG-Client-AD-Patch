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

// Launches the unmodified OP.GG client, applies the ad-free state at runtime,
// then detaches. Nothing on disk is changed.

func main() {
	fmt.Println("OP.GG Launcher - https://github.com/Lv-Max/OP-GG-Client-AD-Patch")

	exe, err := findClient()
	if err != nil {
		fatal(err.Error())
	}
	killRunning()
	fmt.Println("opening OP.GG...")

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			killRunning()
			time.Sleep(2 * time.Second)
		}
		port := randomPort()
		if err := spawn(exe, port); err != nil {
			fatal("failed to start client: " + err.Error())
		}
		if lastErr = inject(port); lastErr == nil {
			fmt.Println("patch applied")
			holdOpen()
			return
		}
	}

	// Never leave the user worse off: the client is running, just unpatched.
	fmt.Println("patch not applied:", lastErr.Error())
	fmt.Println("the client is running normally (no changes made).")
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
	c.send("Runtime.runIfWaitingForDebugger", nil, 5*time.Second) // no-op under --inspect

	// Retry until it takes (the app's modules need to be loaded first).
	deadline := time.Now().Add(15 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		v, err := c.evaluate(installJS, 6*time.Second)
		if err != nil {
			return err
		}
		last = v
		if strings.HasPrefix(v, "ok:") {
			c.evaluate(closeInspectorJS, 3*time.Second)
			return nil
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
	cmd := exec.Command(exe, fmt.Sprintf("--inspect=127.0.0.1:%d", port))
	cmd.Dir = filepath.Dir(exe)
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
