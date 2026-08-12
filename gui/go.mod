module github.com/Bli-AIk/kristal-debug-tools/gui

go 1.26.5

require github.com/jchv/go-webview-selector v0.0.0-20250730141630-a5f64a01ba3a

require (
	github.com/jchv/go-webview2 v0.0.0-20220126073738-2ea27096a5eb // indirect
	github.com/jchv/go-winloader v0.0.0-20200815041850-dec1ee9a7fd5 // indirect
	github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6 // indirect
	golang.org/x/sys v0.0.0-20210218145245-beda7e5e158e // indirect
)

// Vendored webview_go with the Linux backend patched from webkit2gtk-4.0 to
// webkit2gtk-4.1 (the API every current distro ships; upstream still pins
// the orphaned 4.0 ABI). See third_party/webview_go/README-patch.md.
replace github.com/webview/webview_go => ./third_party/webview_go
