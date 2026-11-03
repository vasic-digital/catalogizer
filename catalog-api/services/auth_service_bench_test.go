package services

import (
	"fmt"
	"testing"
	"time"

	"catalogizer/models"
)

func newBenchAuthService() *AuthService {
	return &AuthService{
		jwtSecret:  []byte("bench-test-secret-key-for-benchmarking"),
		jwtExpiry:  24 * time.Hour,
		refreshExp: 7 * 24 * time.Hour,
	}
}

// --- HashPassword benchmarks ---

func BenchmarkHashPassword(b *testing.B) {
	svc := newBenchAuthService()

	benchmarks := []struct {
		name     string
		password string
		salt     string
	}{
		{"short_password", "pass1234", "salt1234"},
		{"medium_password", "a_medium_length_pass!", "medium_salt_1234"},
		{"long_password", "a_very_long_password_that_someone_might_actually_use_in_practice_!@#$", "a_longer_salt_value_here_1234567890"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = svc.hashPassword(bm.password, bm.salt)
			}
		})
	}
}

// --- ValidateToken benchmarks ---

func BenchmarkValidateToken(b *testing.B) {
	svc := newBenchAuthService()

	// Pre-generate tokens for different users
	users := []struct {
		name string
		user *models.User
		sid  int
	}{
		{"standard_user", &models.User{ID: 1, Username: "testuser", RoleID: 1}, 100},
		{"admin_user", &models.User{ID: 42, Username: "admin_user_with_long_name", RoleID: 2}, 999},
	}

	for _, u := range users {
		token, err := svc.generateJWT(u.user, u.sid)
		if err != nil {
			b.Fatalf("failed to generate JWT for %s: %v", u.name, err)
		}

		b.Run(u.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = svc.validateToken(token)
			}
		})
	}

	// Benchmark invalid token validation
	b.Run("invalid_token", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = svc.validateToken("invalid.jwt.token")
		}
	})
}

// --- GenerateJWT benchmarks ---

func BenchmarkGenerateJWT(b *testing.B) {
	svc := newBenchAuthService()

	users := []struct {
		name string
		user *models.User
		sid  int
	}{
		{"standard_user", &models.User{ID: 1, Username: "user1", RoleID: 1}, 1},
		{"admin_user", &models.User{ID: 42, Username: "admin", RoleID: 2}, 999},
	}

	for _, u := range users {
		b.Run(u.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = svc.generateJWT(u.user, u.sid)
			}
		})
	}
}

// --- HashData benchmark ---

// --- GenerateSecureToken benchmark ---

func BenchmarkGenerateSecureToken(b *testing.B) {
	svc := newBenchAuthService()

	lengths := []int{16, 32, 64, 128}

	for _, length := range lengths {
		b.Run(fmt.Sprintf("length=%d", length), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = svc.GenerateSecureToken(length)
			}
		})
	}
}
