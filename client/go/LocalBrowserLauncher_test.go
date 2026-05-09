package modcdp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func TestLocalBrowserLauncherLaunchesRealBrowserAndSpeaksCDP(t *testing.T) {
	headless := true
	sandbox := false
	chrome, err := NewLocalBrowserLauncher(LaunchOptions{
		Headless: &headless,
		Sandbox:  &sandbox,
	}).Launch(LaunchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(ctx, chrome.WSURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := wsutil.WriteClientText(conn, []byte(`{"id":1,"method":"Browser.getVersion","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	body, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		ID     int `json:"id"`
		Result struct {
			Product         string `json:"product"`
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != 1 {
		t.Fatalf("unexpected response id %d", response.ID)
	}
	if !strings.Contains(response.Result.Product, "Chrome") && !strings.Contains(response.Result.Product, "Chromium") {
		t.Fatalf("unexpected product %q", response.Result.Product)
	}
	if response.Result.ProtocolVersion == "" {
		t.Fatal("expected protocolVersion")
	}
}
