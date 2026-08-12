# webview_go (vendored, webkit2gtk-4.1 patch)

Vendored from https://github.com/webview/webview_go at
`v0.0.0-20240831120633-6173450d4dd6` (MIT, © 2017 Serge Zaitsev), with one
local change:

```diff
-#cgo linux openbsd freebsd netbsd pkg-config: gtk+-3.0 webkit2gtk-4.0
+#cgo linux openbsd freebsd netbsd pkg-config: gtk+-3.0 webkit2gtk-4.1
```

**Why:** upstream still pins webkit2gtk-4.0, an ABI that current distros have
moved away from (on some systems the 4.0 provider is orphaned and links a
libjxl soname that no longer exists, making any 4.0-linked binary fail to
link). webkit2gtk-4.1 is the same API with soup3; the backend code has no
soup-specific calls, so the one-line change is sufficient.

**How to re-vendor:** bump the version in `go.mod`, copy the package files
here, re-apply the diff above, and update `gui/go.mod`'s `replace` target if
the directory moved. Windows/macOS builds are untouched by this patch (they
do not use the pkg-config line).
