package repository

import (
	"context"
	"fmt"

	"go-hephaestus/internal/core/domain"
)

// Internal scheduler contexts have no actor; HTTP contexts always carry one.
// Call this before returning credentials to a service, including cached configs.
func RequireConnectionAccess(ctx context.Context, table, shares, column, id, action string) error {
	user := domain.UserFromContext(ctx)
	if user == nil || user.IsAdmin() {
		return nil
	}
	allowed, owner, permission, err := NewConnectionShareRepository().CheckAccess(ctx, table, shares, column, id, user.ID, user.Role)
	if err != nil || !allowed || (action != "read" && !owner && permission != "manage") {
		return fmt.Errorf("access denied to integration configuration")
	}
	return nil
}
