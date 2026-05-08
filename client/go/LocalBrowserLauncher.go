package modcdp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LocalBrowserLauncher struct {
	BrowserLauncher
}

func NewLocalBrowserLauncher(options LaunchOptions) LocalBrowserLauncher {
	return LocalBrowserLauncher{BrowserLauncher: NewBrowserLauncher(options)}
}

func (l LocalBrowserLauncher) Launch(options LaunchOptions) (*LaunchedBrowser, error) {
	if options.ExecutablePath == "" {
		options.ExecutablePath = l.Options.ExecutablePath
	}
	if options.Port == 0 {
		options.Port = l.Options.Port
	}
	if options.UserDataDir == "" {
		options.UserDataDir = l.Options.UserDataDir
	}
	if len(options.ExtraArgs) == 0 {
		options.ExtraArgs = l.Options.ExtraArgs
	}
	if options.Headless == nil {
		options.Headless = l.Options.Headless
	}
	if options.Sandbox == nil {
		options.Sandbox = l.Options.Sandbox
	}

	executablePath, err := findChromeBinary(options.ExecutablePath)
	if err != nil {
		return nil, err
	}
	port := options.Port
	if port == 0 {
		port, err = freePort()
		if err != nil {
			return nil, err
		}
	}
	profileDir := options.UserDataDir
	ownsProfileDir := false
	if profileDir == "" {
		profileDir, err = os.MkdirTemp("", "modcdp.")
		if err != nil {
			return nil, err
		}
		ownsProfileDir = true
	}
	args := []string{
		"--enable-unsafe-extension-debugging",
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-default-apps",
		"--disable-background-networking",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-background-timer-throttling",
		"--disable-dev-shm-usage",
		"--disable-sync",
		"--disable-features=DisableLoadExtensionCommandLineSwitch",
		"--password-store=basic",
		"--use-mock-keychain",
		"--disable-gpu",
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--remote-debugging-address=127.0.0.1",
		fmt.Sprintf("--remote-debugging-port=%d", port),
	}
	headless := runtime.GOOS == "linux" && os.Getenv("DISPLAY") == ""
	if options.Headless != nil {
		headless = *options.Headless
	}
	if headless {
		args = append(args, "--headless=new")
	}
	if options.Sandbox == nil || !*options.Sandbox {
		args = append(args, "--no-sandbox")
	}
	args = append(args, options.ExtraArgs...)
	args = append(args, "about:blank")
	cmd := exec.Command(executablePath, args...)
	if err := cmd.Start(); err != nil {
		if ownsProfileDir {
			_ = os.RemoveAll(profileDir)
		}
		return nil, err
	}
	close := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		if ownsProfileDir {
			_ = os.RemoveAll(profileDir)
		}
	}
	cdpURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(time.Duration(DefaultChromeReadyTimeoutMS) * time.Millisecond)
	for time.Now().Before(deadline) {
		resp, err := http.Get(cdpURL + "/json/version")
		if err == nil {
			var version map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&version)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				wsURL, _ := version["webSocketDebuggerUrl"].(string)
				return &LaunchedBrowser{CDPURL: cdpURL, WSURL: wsURL, Close: close, ProfileDir: profileDir}, nil
			}
		}
		time.Sleep(time.Duration(DefaultServiceWorkerPollIntervalMS) * time.Millisecond)
	}
	close()
	return nil, fmt.Errorf("Chrome at %s did not become ready within %dms", cdpURL, DefaultChromeReadyTimeoutMS)
}

func findChromeBinary(explicit string) (string, error) {
	candidates := append([]string{explicit, os.Getenv("CHROME_PATH")}, candidatePaths()...)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	tried := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate != "" {
			tried = append(tried, candidate)
		}
	}
	return "", fmt.Errorf("no Chrome/Chromium binary found. Tried: %s. Set CHROME_PATH or pass Launch.Options.ExecutablePath", strings.Join(tried, ", "))
}

func candidatePaths() []string {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "."
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}
	programFiles := compact([]string{os.Getenv("PROGRAMFILES"), os.Getenv("PROGRAMFILES(X86)")})

	var canary []string
	var stock []string
	switch runtime.GOOS {
	case "darwin":
		canary = []string{"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"}
		stock = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
	case "windows":
		canary = append([]string{filepath.Join(localAppData, "Google", "Chrome SxS", "Application", "chrome.exe")}, joinAll(programFiles, "Google", "Chrome SxS", "Application", "chrome.exe")...)
		stock = append(joinAll(programFiles, "Google", "Chrome", "Application", "chrome.exe"), filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"))
	default:
		canary = []string{"/usr/bin/google-chrome-canary", "/usr/bin/google-chrome-unstable", "/opt/google/chrome-unstable/chrome"}
		stock = []string{"/usr/bin/google-chrome-stable", "/usr/bin/google-chrome", "/opt/google/chrome/chrome"}
	}

	result := append([]string{}, canary...)
	result = append(result, chromeForTestingCandidates()...)
	result = append(result, stock...)
	return result
}

func chromeForTestingCandidates() []string {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "."
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}

	var patterns []string
	switch runtime.GOOS {
	case "darwin":
		patterns = []string{
			filepath.Join(homeDir, "Library", "Caches", "ms-playwright", "chromium-*", "chrome-mac*", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
			filepath.Join(homeDir, "Library", "Caches", "ms-playwright", "chromium-*", "chrome-mac*", "Chromium.app", "Contents", "MacOS", "Chromium"),
			filepath.Join(homeDir, "Library", "Caches", "puppeteer", "chrome", "mac*-*", "chrome-mac*", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
		}
	case "windows":
		patterns = []string{
			filepath.Join(localAppData, "ms-playwright", "chromium-*", "chrome-win*", "chrome.exe"),
			filepath.Join(homeDir, ".cache", "puppeteer", "chrome", "win*-*", "chrome-win*", "chrome.exe"),
		}
	default:
		patterns = []string{
			filepath.Join(homeDir, ".cache", "ms-playwright", "chromium-*", "chrome-linux*", "chrome"),
			filepath.Join("/opt", "pw-browsers", "chromium-*", "chrome-linux*", "chrome"),
			filepath.Join(homeDir, ".cache", "puppeteer", "chrome", "linux-*", "chrome-linux*", "chrome"),
		}
	}

	var candidates []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		candidates = append(candidates, matches...)
	}
	return newestFirst(candidates)
}

func newestFirst(candidates []string) []string {
	seen := map[string]bool{}
	deduped := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		deduped = append(deduped, candidate)
	}
	sort.SliceStable(deduped, func(i, j int) bool {
		leftVersion := maxNumber(deduped[i])
		rightVersion := maxNumber(deduped[j])
		if leftVersion != rightVersion {
			return leftVersion > rightVersion
		}
		leftStat, leftErr := os.Stat(deduped[i])
		rightStat, rightErr := os.Stat(deduped[j])
		var leftMtime, rightMtime time.Time
		if leftErr == nil {
			leftMtime = leftStat.ModTime()
		}
		if rightErr == nil {
			rightMtime = rightStat.ModTime()
		}
		if !leftMtime.Equal(rightMtime) {
			return leftMtime.After(rightMtime)
		}
		return deduped[i] < deduped[j]
	})
	return deduped
}

func maxNumber(value string) int {
	max := 0
	for _, raw := range regexp.MustCompile(`\d+`).FindAllString(value, -1) {
		number, err := strconv.Atoi(raw)
		if err == nil && number > max {
			max = number
		}
	}
	return max
}

func compact(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func joinAll(bases []string, parts ...string) []string {
	result := make([]string, 0, len(bases))
	for _, base := range bases {
		result = append(result, filepath.Join(append([]string{base}, parts...)...))
	}
	return result
}
