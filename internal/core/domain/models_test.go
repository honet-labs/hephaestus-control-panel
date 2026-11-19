package domain

import "testing"

func TestUsernameDoesNotGrantAdmin(t *testing.T) {
	for _, name := range []string{"admin", "administrator", "root"} {
		user := &User{Username: name, Role: "VIEWER", Permissions: map[string]string{"opensearch": "read"}}
		if user.IsAdmin() || user.Can("opensearch", "manage") || user.Can("users", "read") {
			t.Fatalf("username %q granted privileges", name)
		}
		if !user.Can("opensearch", "read") {
			t.Fatal("read grant not respected")
		}
	}
	var absent *User
	if absent.Can("users", "manage") || absent.IsAdmin() {
		t.Fatal("nil user granted privileges")
	}
	if !(&User{Role: "ADMIN"}).Can("users", "manage") {
		t.Fatal("admin role lost privileges")
	}
}
