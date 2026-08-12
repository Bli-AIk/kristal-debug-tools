// Command kristal-debug-tools-gui is the Windows-friendly GUI for the
// library: a local HTTP server (embedded frontend + API) shown in a single
// webview window, with the system browser as fallback (also useful for
// browser DevTools during development: pass --no-window).
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/discover"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/launcher"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/server"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/window"
)

func main() {
	var (
		modRootFlag   = flag.String("mod-root", "", "Kristal mod root (default: walk up from the working directory)")
		justfileFlag  = flag.String("justfile", "", "path to the kristal-debug-tools justfile")
		portFlag      = flag.Int("port", 0, "port to listen on (0 = random)")
		noWindowFlag  = flag.Bool("no-window", false, "do not open a webview window; print the URL and open the system browser")
		verboseFlag   = flag.Bool("verbose", false, "log HTTP requests")
	)
	flag.Parse()

	modRoot, engineRoot, modID := discover.ModRoot(discover.EnvOr(*modRootFlag, "KDT_MOD_ROOT"))
	justfile := discover.Justfile(discover.EnvOr(*justfileFlag, "KDT_JUSTFILE"), modRoot)

	justPath, justMode, justVersion := server.ResolveJust(filepath.Dir(justfile))
	lovePath, _ := launcher.LookupLove()

	s := server.New(server.Options{
		ModRoot: modRoot, ModID: modID, EngineRoot: engineRoot,
		Justfile: justfile, JustPath: justPath, JustMode: justMode, JustVersion: justVersion,
		LovePath: lovePath,
	})
	var handler http.Handler = s.Handler()
	if *verboseFlag {
		handler = logRequests(handler)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(*portFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
	url := "http://" + ln.Addr().String()
	fmt.Printf("kristal-debug-tools GUI: %s\n", url)

	srv := &http.Server{Handler: handler}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *noWindowFlag {
		// Debug mode: browser (DevTools available), server lives until ^C.
		window.OpenBrowser(url)
		fmt.Println("browser opened (press Ctrl+C to stop)")
		waitForSignalOrServe(ctx, srv, serveDone)
		return
	}

	if err := window.Open(url, "Kristal Debug Tools", 1100, 720); err != nil {
		// No webview backend (e.g. missing WebView2 runtime): fall back to
		// the browser and keep serving so the app still works.
		fmt.Printf("webview window unavailable (%v) — opening the browser instead\n", err)
		window.OpenBrowser(url)
		waitForSignalOrServe(ctx, srv, serveDone)
		return
	}
	// Window closed: quit the app.
	srv.Close()
	<-serveDone
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func waitForSignalOrServe(ctx context.Context, srv *http.Server, serveDone <-chan error) {
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	case <-serveDone:
	}
}
