package integration_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// Keep feature authorization on every protected registration, including aliases.
// Resource-specific checks are tested separately against the live database.
func TestProtectedRoutePermissionCoverage(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "../../cmd/server/main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || receiver.Name != "api" {
			return true
		}
		switch selector.Sel.Name {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			return true
		}
		count++
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			t.Error("nonliteral API path requires review")
			return true
		}
		path, _ := strconv.Unquote(literal.Value)
		if path == "/auth/me" || path == "/auth/change-password" {
			return true
		}
		guarded := false
		for _, argument := range call.Args[1:] {
			middleware, ok := argument.(*ast.CallExpr)
			if !ok {
				continue
			}
			name, ok := middleware.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if name.Sel.Name == "RequirePermission" || name.Sel.Name == "RequireAnyPermission" || name.Sel.Name == "RequireRole" {
				guarded = true
			}
		}
		if !guarded {
			t.Errorf("%s %s has no feature/role authorization", selector.Sel.Name, path)
		}
		return true
	})
	if count < 200 {
		t.Fatalf("route scan unexpectedly covered only %d endpoints", count)
	}
	t.Logf("checked feature/role authorization on %d protected route registrations", count)
}
