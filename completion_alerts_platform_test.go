package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestAppNativeCompletionStartupAndEnableCallsites(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct{ method, call string }{
		{"startup", "nativeCompletionStart"},
		{"SetCompletionAlerts", "nativeCompletionEnable"},
	} {
		t.Run(expected.method, func(t *testing.T) {
			found := false
			for _, declaration := range file.Decls {
				method, ok := declaration.(*ast.FuncDecl)
				if !ok || method.Name.Name != expected.method || method.Recv == nil {
					continue
				}
				ast.Inspect(method.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					name, ok := call.Fun.(*ast.Ident)
					if ok && name.Name == expected.call {
						found = true
					}
					return true
				})
			}
			if !found {
				t.Errorf("App.%s no longer calls %s", expected.method, expected.call)
			}
		})
	}
}

// These source pins identify the native mechanisms that a headless test cannot
// visibly observe. The native build jobs compile each adapter separately.
func TestCompletionAlertPlatformMechanisms(t *testing.T) {
	for _, platform := range []struct {
		name string
		file string
		need []string
	}{
		{"macOS", "completion_alerts_darwin.go", []string{
			"dispatch_async(dispatch_get_main_queue(), ^{\n        @autoreleasepool", "NSInformationalRequest", "[NSApp isActive]",
			"pthread_mutex_lock(&fairdropAlertQueueLock)", "fairdropPendingFocus", "fairdropAlertEpoch",
		}},
		{"Windows", "completion_alerts_windows.go", []string{
			"FlashWindowEx", "flashTray = 0x00000002", "count: 3", "wailsWindow",
			"alertGetWindowPID", "alertGetForeground", "alertIsWindowVisible", "alertEpoch.Load() != request.epoch",
		}},
	} {
		t.Run(platform.name, func(t *testing.T) {
			content, err := os.ReadFile(platform.file)
			if err != nil {
				t.Fatal(err)
			}
			normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
			for _, needle := range platform.need {
				if !strings.Contains(normalized, needle) {
					t.Errorf("%s no longer contains %q", platform.file, needle)
				}
			}
		})
	}
}
