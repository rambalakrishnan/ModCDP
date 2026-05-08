package modcdp

import "fmt"

type BrowserbaseBrowserLauncher struct {
	BrowserLauncher
}

func NewBrowserbaseBrowserLauncher(options LaunchOptions) BrowserbaseBrowserLauncher {
	return BrowserbaseBrowserLauncher{BrowserLauncher: NewBrowserLauncher(options)}
}

func (l BrowserbaseBrowserLauncher) Launch(options LaunchOptions) (*LaunchedBrowser, error) {
	return nil, fmt.Errorf("launch.mode=bb is not implemented yet")
}
