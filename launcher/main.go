package main

import (
	"crypto/rand"
	"encoding/json"
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
	if killRunning() {
		time.Sleep(2 * time.Second)
	}

	port := randomPort()
	fmt.Println("opening OP.GG...")
	if err := spawn(exe, port); err != nil {
		fatal("failed to start client: " + err.Error())
	}

	if err := injectWithRetry(port); err != nil {
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

func injectWithRetry(port int) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := inject(port); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
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

	if err := grabWebpackRequire(c); err != nil {
		return fmt.Errorf("reach app internals: %w", err)
	}

	// Install the hook, retrying until the store module is loaded.
	deadline := time.Now().Add(15 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		v, err := c.evaluate(installJS, 5*time.Second)
		if err != nil {
			return err
		}
		last = v
		if strings.HasPrefix(v, "ok:") {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !strings.HasPrefix(last, "ok:") {
		return fmt.Errorf("hook did not take (last: %q)", last)
	}

	// Re-assert briefly so the value is present before the renderer reads it.
	for i := 0; i < 12; i++ {
		c.evaluate(installJS, 3*time.Second)
		time.Sleep(250 * time.Millisecond)
	}

	// Shut the debug port now that we are done.
	c.evaluate(closeInspectorJS, 3*time.Second)
	return nil
}

// grabWebpackRequire stashes the bundle's private __webpack_require__ on
// globalThis by breaking inside it once, then removing the breakpoint.
func grabWebpackRequire(c *cdpClient) error {
	if _, err := c.send("Runtime.enable", nil, 5*time.Second); err != nil {
		return err
	}
	if _, err := c.send("Debugger.enable", nil, 5*time.Second); err != nil {
		return err
	}

	// Find main.js among the (replayed) scriptParsed events.
	scripts := map[string]string{}
	mainID := ""
	deadline := time.Now().Add(15 * time.Second)
	for mainID == "" && time.Now().Before(deadline) {
		ev, ok := waitEvent(c, 15*time.Second)
		if !ok {
			break
		}
		if ev.Method == "Debugger.scriptParsed" {
			var p struct {
				ScriptID string `json:"scriptId"`
				URL      string `json:"url"`
			}
			if json.Unmarshal(ev.Params, &p) == nil {
				scripts[p.ScriptID] = p.URL
				if strings.HasSuffix(p.URL, "assets/main/main.js") {
					mainID = p.ScriptID
				}
			}
		}
	}
	if mainID == "" {
		return fmt.Errorf("main.js not found")
	}

	// Fetch source, locate the require function body.
	res, err := c.send("Debugger.getScriptSource", map[string]any{"scriptId": mainID}, 15*time.Second)
	if err != nil {
		return err
	}
	var src struct {
		ScriptSource string `json:"scriptSource"`
	}
	if json.Unmarshal(res, &src) != nil || src.ScriptSource == "" {
		return fmt.Errorf("empty script source")
	}
	idx := strings.Index(src.ScriptSource, webpackRequireAnchor)
	if idx < 0 {
		return fmt.Errorf("webpack require anchor not found (client updated?)")
	}
	col := idx + len(webpackRequireAnchor)

	bp, err := c.send("Debugger.setBreakpoint", map[string]any{
		"location": map[string]any{"scriptId": mainID, "lineNumber": 0, "columnNumber": col},
	}, 10*time.Second)
	if err != nil {
		return err
	}
	var bpRes struct {
		BreakpointID string `json:"breakpointId"`
	}
	json.Unmarshal(bp, &bpRes)

	// Wait for the breakpoint to hit; grab the require in that frame.
	grabbed := false
	deadline = time.Now().Add(15 * time.Second)
	for !grabbed && time.Now().Before(deadline) {
		ev, ok := waitEvent(c, 15*time.Second)
		if !ok {
			break
		}
		if ev.Method != "Debugger.paused" {
			continue
		}
		var p struct {
			HitBreakpoints []string `json:"hitBreakpoints"`
			CallFrames     []struct {
				CallFrameID string `json:"callFrameId"`
			} `json:"callFrames"`
		}
		if json.Unmarshal(ev.Params, &p) != nil || len(p.CallFrames) == 0 {
			c.send("Debugger.resume", nil, 3*time.Second)
			continue
		}
		if len(p.HitBreakpoints) == 0 {
			c.send("Debugger.resume", nil, 3*time.Second)
			continue
		}
		_, err := c.send("Debugger.evaluateOnCallFrame", map[string]any{
			"callFrameId":   p.CallFrames[0].CallFrameID,
			"expression":    "(globalThis.__opgg_wr=__webpack_require__),'ok'",
			"returnByValue": true,
		}, 5*time.Second)
		if err != nil {
			return err
		}
		grabbed = true
		if bpRes.BreakpointID != "" {
			c.send("Debugger.removeBreakpoint", map[string]any{"breakpointId": bpRes.BreakpointID}, 3*time.Second)
		}
		c.send("Debugger.resume", nil, 3*time.Second)
		c.send("Debugger.disable", nil, 3*time.Second)
	}
	if !grabbed {
		return fmt.Errorf("breakpoint never hit")
	}
	return nil
}

func waitEvent(c *cdpClient, timeout time.Duration) (cdpEvent, bool) {
	select {
	case ev, ok := <-c.events:
		return ev, ok
	case <-time.After(timeout):
		return cdpEvent{}, false
	}
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
