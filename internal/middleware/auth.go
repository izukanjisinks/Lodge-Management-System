package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"lodge-system/internal/repository"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type ContextKey string

const (
	UserIDKey           ContextKey = "userID"
	UserEmail           ContextKey = "userEmail"
	UserKey             ContextKey = "user"
	OrgIDKey            ContextKey = "orgID"
	BranchIDKey         ContextKey = "branchID"
	RoleKey             ContextKey = "role"
	BackofficeUserIDKey ContextKey = "backofficeUserID"
)

// JWTAuth validates a staff JWT and injects user, org_id and role into context.
func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := extractAndValidateClaims(w, r)
		if !ok {
			return
		}

		if claims.TokenType != utils.TokenTypeStaff {
			utils.RespondError(w, http.StatusUnauthorized, "Invalid token type for this endpoint")
			return
		}

		userRepo := repository.NewUserRepository()
		user, err := userRepo.GetUserByID(claims.UserID)
		if err != nil {
			utils.RespondError(w, http.StatusUnauthorized, "User not found")
			return
		}

		if !user.IsActive {
			utils.RespondError(w, http.StatusUnauthorized, "Account is inactive")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmail, claims.Email)
		ctx = context.WithValue(ctx, UserKey, user)
		ctx = context.WithValue(ctx, OrgIDKey, claims.OrgID)
		ctx = context.WithValue(ctx, BranchIDKey, claims.BranchID)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GuestJWTAuth validates a guest JWT and injects guest_id into context.
func GuestJWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := extractAndValidateClaims(w, r)
		if !ok {
			return
		}

		if claims.TokenType != utils.TokenTypeGuest {
			utils.RespondError(w, http.StatusUnauthorized, "Invalid token type for this endpoint")
			return
		}

		guestRepo := repository.NewGuestRepository()
		guest, err := guestRepo.GetByID(claims.UserID)
		if err != nil {
			utils.RespondError(w, http.StatusUnauthorized, "Guest not found")
			return
		}

		if !guest.IsActive {
			utils.RespondError(w, http.StatusUnauthorized, "Account is inactive")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmail, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WebUserJWTAuth validates a web-user JWT (token type "guest") and injects user_id into context.
func WebUserJWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := extractAndValidateClaims(w, r)
		if !ok {
			return
		}

		if claims.TokenType != utils.TokenTypeGuest {
			utils.RespondError(w, http.StatusUnauthorized, "Invalid token type for this endpoint")
			return
		}

		webUserRepo := repository.NewWebUserRepository()
		u, err := webUserRepo.GetByID(claims.UserID)
		if err != nil {
			utils.RespondError(w, http.StatusUnauthorized, "User not found")
			return
		}

		if !u.IsActive {
			utils.RespondError(w, http.StatusUnauthorized, "Account is inactive")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmail, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BackofficeJWTAuth validates a backoffice JWT and injects backoffice_user_id into context.
func BackofficeJWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := extractAndValidateClaims(w, r)
		if !ok {
			return
		}

		if claims.TokenType != utils.TokenTypeBackoffice {
			utils.RespondError(w, http.StatusUnauthorized, "Invalid token type for this endpoint")
			return
		}

		backofficeRepo := repository.NewBackofficeUserRepository()
		bu, err := backofficeRepo.GetByID(claims.UserID)
		if err != nil {
			utils.RespondError(w, http.StatusUnauthorized, "Backoffice user not found")
			return
		}

		if !bu.IsActive {
			utils.RespondError(w, http.StatusUnauthorized, "Account is inactive")
			return
		}

		ctx := context.WithValue(r.Context(), BackofficeUserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmail, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractAndValidateClaims(w http.ResponseWriter, r *http.Request) (*utils.CustomClaims, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		utils.RespondError(w, http.StatusUnauthorized, "Missing authorization header")
		return nil, false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		utils.RespondError(w, http.StatusUnauthorized, "Invalid authorization header format. Expected: Bearer <token>")
		return nil, false
	}

	claims, err := utils.ValidateToken(parts[1])
	if err != nil {
		utils.RespondError(w, http.StatusUnauthorized, "Invalid or expired token")
		return nil, false
	}

	return claims, true
}

func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

func GetUserEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmail).(string)
	return email, ok
}

func GetOrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	orgID, ok := ctx.Value(OrgIDKey).(uuid.UUID)
	return orgID, ok
}

func GetBranchIDFromContext(ctx context.Context) *uuid.UUID {
	v := ctx.Value(BranchIDKey)
	if v == nil {
		return nil
	}
	id, ok := v.(*uuid.UUID)
	if !ok {
		return nil
	}
	return id
}

// ResolveBranchID returns the branch_id to use for a scoped query.
// Branch-scoped staff have it fixed in their JWT; org-level admins may
// optionally supply it as a query param.
func ResolveBranchID(r *http.Request) (*uuid.UUID, error) {
	if id := GetBranchIDFromContext(r.Context()); id != nil {
		return id, nil
	}
	if v := r.URL.Query().Get("branch_id"); v != "" {
		parsed, err := uuid.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid branch_id")
		}
		return &parsed, nil
	}
	return nil, nil
}

func GetRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleKey).(string)
	return role, ok
}

func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmail).(string)
	return email, ok
}

func GetBackofficeUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(BackofficeUserIDKey).(uuid.UUID)
	return id, ok
}
