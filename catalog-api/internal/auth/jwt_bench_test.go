package auth

import (
	"testing"

	"catalogizer/database"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// setupBenchAuthService creates an AuthService backed by a no-op sqlmock
// database, suitable for benchmarking token generation and validation
// without database I/O.
func setupBenchAuthService(b *testing.B) *AuthService {
	b.Helper()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		b.Fatalf("failed to create sqlmock: %v", err)
	}
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()
	return NewAuthService(db, "bench-secret-key-for-jwt-testing", logger)
}

// BenchmarkGenerateToken measures the cost of generating a signed JWT
// access token using digital.vasic.auth/pkg/jwt.Manager. This is called
// on every successful login.
func BenchmarkGenerateToken(b *testing.B) {
	service := setupBenchAuthService(b)
	user := &User{
		ID:       1,
		Username: "benchuser",
		Role:     RoleUser,
		Permissions: []string{
			PermissionReadMedia, PermissionWriteMedia,
			PermissionReadCatalog, PermissionWriteCatalog,
			PermissionViewAnalysis, PermissionAPIAccess,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.generateToken(user, "access")
		if err != nil {
			b.Fatalf("generateToken failed: %v", err)
		}
	}
}

// BenchmarkGenerateRefreshToken measures refresh token generation, which
// uses a separate jwt.Manager with 7-day TTL.
func BenchmarkGenerateRefreshToken(b *testing.B) {
	service := setupBenchAuthService(b)
	user := &User{
		ID:       1,
		Username: "benchuser",
		Role:     RoleAdmin,
		Permissions: GetRolePermissions(RoleAdmin),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.generateToken(user, "refresh")
		if err != nil {
			b.Fatalf("generateToken (refresh) failed: %v", err)
		}
	}
}

// BenchmarkValidateToken measures the cost of parsing and validating an
// existing JWT token using jwt.ParseWithClaims. This runs on every
// authenticated API request.
func BenchmarkValidateToken(b *testing.B) {
	service := setupBenchAuthService(b)
	user := &User{
		ID:       1,
		Username: "benchuser",
		Role:     RoleUser,
		Permissions: []string{
			PermissionReadMedia, PermissionReadCatalog,
			PermissionViewAnalysis, PermissionAPIAccess,
		},
	}

	token, err := service.generateToken(user, "access")
	if err != nil {
		b.Fatalf("setup: generateToken failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.validateToken(token)
		if err != nil {
			b.Fatalf("validateToken failed: %v", err)
		}
	}
}

// BenchmarkHashPassword measures bcrypt password hashing at the default
// cost. This is called during user registration and password changes.
// bcrypt is intentionally slow, so this benchmark documents the baseline
// cost per hash operation.
func BenchmarkHashPassword(b *testing.B) {
	password := []byte("benchmark-password-2024")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
		if err != nil {
			b.Fatalf("GenerateFromPassword failed: %v", err)
		}
	}
}

// BenchmarkVerifyPassword measures bcrypt password verification against a
// pre-computed hash. This is the hot path during login.
func BenchmarkVerifyPassword(b *testing.B) {
	password := []byte("benchmark-password-2024")
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		b.Fatalf("setup: GenerateFromPassword failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := bcrypt.CompareHashAndPassword(hash, password)
		if err != nil {
			b.Fatalf("CompareHashAndPassword failed: %v", err)
		}
	}
}

// BenchmarkGetRolePermissions measures the cost of looking up permissions
// for a role. This is called on every token validation to populate user
// permissions.
func BenchmarkGetRolePermissions(b *testing.B) {
	roles := []string{RoleAdmin, RoleModerator, RoleUser, RoleViewer}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GetRolePermissions(roles[i%len(roles)])
	}
}

// BenchmarkHasPermission measures the cost of checking whether a user
// holds a specific permission. This linear scan is the authorization
// hot path.
func BenchmarkHasPermission(b *testing.B) {
	user := &User{
		Permissions: GetRolePermissions(RoleAdmin),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = user.HasPermission(PermissionAPIAccess)
	}
}
