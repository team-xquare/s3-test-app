package auth

import "context"

// Role represents user roles
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleUploader Role = "uploader"
	RoleViewer   Role = "viewer"
)

// User represents an authenticated user
type User struct {
	ID    string
	Name  string
	Email string
	Role  Role
}

// Permission represents what actions a role can perform
type Permission struct {
	CanUpload bool
	CanView   bool
	CanDelete bool
	CanManage bool
}

// PermissionMap defines permissions for each role
var PermissionMap = map[Role]Permission{
	RoleAdmin: {
		CanUpload: true,
		CanView:   true,
		CanDelete: true,
		CanManage: true,
	},
	RoleUploader: {
		CanUpload: true,
		CanView:   true,
		CanDelete: false,
		CanManage: false,
	},
	RoleViewer: {
		CanUpload: false,
		CanView:   true,
		CanDelete: false,
		CanManage: false,
	},
}

// ContextKey for storing user in context
type ContextKey string

const UserContextKey ContextKey = "user"

// GetUserFromContext retrieves user from context
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}
	return nil
}

// SetUserInContext stores user in context
func SetUserInContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// HasPermission checks if a user has a specific permission
func (u *User) HasPermission(perm Permission) bool {
	if perm.CanUpload && !PermissionMap[u.Role].CanUpload {
		return false
	}
	if perm.CanView && !PermissionMap[u.Role].CanView {
		return false
	}
	if perm.CanDelete && !PermissionMap[u.Role].CanDelete {
		return false
	}
	if perm.CanManage && !PermissionMap[u.Role].CanManage {
		return false
	}
	return true
}