package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "helmix-auth-user"

// JWTMiddleware validates bearer tokens and injects the user into the request context.
func JWTMiddleware(publicKeyPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			rawToken := strings.TrimSpace(authorization[len("Bearer "):])
			user, err := ParseUserToken(publicKeyPath, rawToken)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid bearer token")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole ensures the authenticated user has one of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowedRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		trimmedRole := strings.TrimSpace(role)
		if trimmedRole != "" {
			allowedRoles[trimmedRole] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if _, ok := allowedRoles[user.Role]; !ok {
				writeJSONError(w, http.StatusForbidden, "insufficient role")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UserFromContext returns the authenticated user stored by JWTMiddleware.
func UserFromContext(ctx context.Context) *User {
	user, ok := ctx.Value(userContextKey).(*User)
	if !ok {
		return nil
	}
	return user
}

// ContextWithUser stores an authenticated user in the supplied context.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
