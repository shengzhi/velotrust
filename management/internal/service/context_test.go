package service

import (
	"context"
	"testing"
)

func TestContextCopiesRolesAndExposesRequestData(t *testing.T) {
	roles := []string{"ADMIN", "AUDITOR"}
	ctx := NewContext(context.Background(), Principal{ID: "u-1", Name: "Alice", Roles: roles}, RequestMeta{Language: "zh-CN", RequestID: "r-1", ClientIP: "127.0.0.1"})
	roles[0] = "MUTATED"
	got := ctx.Roles()
	got[1] = "MUTATED"
	if !ctx.HasRole("ADMIN") || ctx.UserID() != "u-1" || ctx.UserName() != "Alice" || ctx.Language() != "zh-CN" || ctx.RequestID() != "r-1" || ctx.ClientIP() != "127.0.0.1" || ctx.HasRole("MUTATED") {
		t.Fatalf("context = %+v", ctx)
	}
}
