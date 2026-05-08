package modcdp

type NoopBrowserLauncher struct {
	BrowserLauncher
}

func NewNoopBrowserLauncher(options LaunchOptions) NoopBrowserLauncher {
	return NoopBrowserLauncher{BrowserLauncher: NewBrowserLauncher(options)}
}

func (l NoopBrowserLauncher) Launch(options LaunchOptions) (*LaunchedBrowser, error) {
	return &LaunchedBrowser{Close: func() {}}, nil
}
