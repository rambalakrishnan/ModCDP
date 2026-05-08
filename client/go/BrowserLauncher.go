package modcdp

import "fmt"

type LaunchedBrowser struct {
	CDPURL     string
	WSURL      string
	Close      func()
	ProfileDir string
}

type BrowserLauncher struct {
	Options LaunchOptions
}

func NewBrowserLauncher(options LaunchOptions) BrowserLauncher {
	return BrowserLauncher{Options: options}
}

func (l BrowserLauncher) Launch(options LaunchOptions) (*LaunchedBrowser, error) {
	return nil, fmt.Errorf("%T.Launch is not implemented", l)
}
