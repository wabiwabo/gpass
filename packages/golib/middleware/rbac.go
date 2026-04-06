package middleware

import (
	"net/http"
)

// Role represents an authorization role.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleUser      Role = "user"
	RoleService   Role = "service"
	RoleDeveloper Role = "developer"
)

// RoleHierarchy defines which roles include which other roles.
// admin includes all roles, service and developer include user, user includes none.
var RoleHierarchy = map[Role][]Role{
	RoleAdmin:     {RoleUser, RoleDeveloper, RoleService},
	RoleService:   {RoleUser},
	RoleDeveloper: {RoleUser},
	RoleUser:      {},
}

// HasRole checks if the given role satisfies the required role
// (either directly or through hierarchy).
func HasRole(userRole, requiredRole Role) bool {
	if userRole == requiredRole {
		return true
	}
	for _, inherited := range RoleHierarchy[userRole] {
		if inherited == requiredRole {
			return true
		}
	}
	return false
}

// RequireRole returns middleware that checks the X-User-Role header
// against the required role, respecting the role hierarchy.
// Returns 401 if the header is missing or empty.
// Returns 403 if the role does not satisfy the requirement.
func RequireRole(required Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleHeader := r.Header.Get("X-User-Role")
			if roleHeader == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userRole := Role(roleHeader)
			if !HasRole(userRole, required) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
