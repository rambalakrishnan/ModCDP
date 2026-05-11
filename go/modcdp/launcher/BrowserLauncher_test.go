package launcher

import (
	"strings"
	"testing"
)

func TestBrowserLauncherMergesLaunchConfigAndExposesTransportAndInjectorConfig(t *testing.T) {
	launcher := NewBrowserLauncher(LaunchOptions{
		CDPURL:            "ws://127.0.0.1:9222/devtools/browser/initial",
		UserDataDir:       "/tmp/modcdp-browser-launcher",
		BrowserbaseAPIKey: "test-key",
		ExtensionID:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Args:              []string{"--load-extension=/tmp/args-one"},
		ExtraArgs:         []string{"--load-extension=/tmp/one"},
	})
	launcher.Update(LaunchOptions{
		CDPURL:    "ws://127.0.0.1:9222/devtools/browser/updated",
		Args:      []string{"--load-extension=/tmp/args-two", "--lang=en-US"},
		ExtraArgs: []string{"--load-extension=/tmp/two", "--window-size=900,700"},
	})

	assertStringsEqual(t, launcher.Options.Args, []string{"--lang=en-US", "--load-extension=/tmp/args-one,/tmp/args-two"})
	assertStringsEqual(t, launcher.Options.ExtraArgs, []string{"--window-size=900,700", "--load-extension=/tmp/one,/tmp/two"})

	transportConfig := launcher.GetTransportConfig()
	if transportConfig["cdp_url"] != "ws://127.0.0.1:9222/devtools/browser/updated" {
		t.Fatalf("cdp_url = %v", transportConfig["cdp_url"])
	}
	if transportConfig["user_data_dir"] != "/tmp/modcdp-browser-launcher" {
		t.Fatalf("user_data_dir = %v", transportConfig["user_data_dir"])
	}

	injectorConfig := launcher.GetInjectorConfig()
	if injectorConfig.BrowserbaseAPIKey != "test-key" {
		t.Fatalf("BrowserbaseAPIKey = %v", injectorConfig.BrowserbaseAPIKey)
	}
	if injectorConfig.ExtensionID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("ExtensionID = %v", injectorConfig.ExtensionID)
	}

	if _, err := launcher.Launch(LaunchOptions{}); err == nil || !strings.Contains(err.Error(), "BrowserLauncher.Launch is not implemented") {
		t.Fatalf("Launch error = %v", err)
	}
}

func assertStringsEqual(t *testing.T, actual []string, expected []string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("len(%v) != len(%v)", actual, expected)
	}
	for index := range actual {
		if actual[index] != expected[index] {
			t.Fatalf("index %d: %q != %q in %v", index, actual[index], expected[index], actual)
		}
	}
}
