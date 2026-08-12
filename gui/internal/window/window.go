// Package window opens the GUI in a native webview window, falling back to
// the system browser when no webview backend is available.
//
// The webview is a thin shell: it only displays the URL served by the local
// HTTP server (see internal/server). No JS<->Go binding is used — all
// communication goes through the HTTP/SSE API, so swapping the backend (or
// dropping the window entirely) is a one-line change.
package window

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/jchv/go-webview-selector"
)

// Open shows a single webview window with the given title and size, loading
// url. It blocks until the window is closed. Returns an error if no webview
// backend could be created (e.g. missing WebView2 runtime on Windows, or no
// display server on Linux).
func Open(url, title string, width, height int) error {
	w := webview.New(false)
	if w == nil {
		return fmt.Errorf("webview backend unavailable on %s", runtime.GOOS)
	}
	defer w.Destroy()
	w.SetTitle(title)
	w.SetSize(width, height, webview.HintNone)
	w.Navigate(url)
	w.Run()
	return nil
}

// OpenBrowser opens url in the system browser. Best effort: the printed URL
// remains the fallback when this fails.
func OpenBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		// cmd /c start "" <url> — the empty quoted string is the (optional)
		// window title; without it a url starting with a quote breaks cmd.
		exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}
