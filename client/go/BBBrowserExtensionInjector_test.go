package modcdp

import "testing"

func TestBBBrowserExtensionInjectorUsesConfiguredExtensionID(t *testing.T) {
	injector := NewBBBrowserExtensionInjector(ExtensionInjectorConfig{
		ExtensionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err := injector.Prepare(); err != nil {
		t.Fatal(err)
	}
	launchConfig := injector.GetLauncherConfig()
	if launchConfig.ExtensionID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("ExtensionID = %q", launchConfig.ExtensionID)
	}
}
