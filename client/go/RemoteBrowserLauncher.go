package modcdp

import "fmt"

type RemoteBrowserLauncher struct {
	BrowserLauncher
	CDPURL string
}

func NewRemoteBrowserLauncher(options LaunchOptions, cdpURL string) RemoteBrowserLauncher {
	return RemoteBrowserLauncher{BrowserLauncher: NewBrowserLauncher(options), CDPURL: cdpURL}
}

func (l RemoteBrowserLauncher) Launch(options LaunchOptions) (*LaunchedBrowser, error) {
	if l.CDPURL == "" {
		return nil, fmt.Errorf("launch.mode=remote requires upstream.ws_url")
	}
	return &LaunchedBrowser{CDPURL: l.CDPURL, WSURL: l.CDPURL, Close: func() {}}, nil
}
