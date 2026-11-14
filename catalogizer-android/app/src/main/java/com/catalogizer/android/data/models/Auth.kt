package com.catalogizer.android.data.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Represents an authenticated user with profile data, role, and permissions.
 */
@Serializable
data class User(
    val id: Long,
    val username: String,
    val email: String,
    @SerialName("first_name")
    val firstName: String,
    @SerialName("last_name")
    val lastName: String,
    val role: String,
    @SerialName("is_active")
    val isActive: Boolean,
    @SerialName("last_login")
    val lastLogin: String? = null,
    @SerialName("created_at")
    val createdAt: String,
    @SerialName("updated_at")
    val updatedAt: String,
    val permissions: List<String>? = null
) {
    val fullName: String
        get() = "$firstName $lastName".trim()

    val isAdmin: Boolean
        get() = role == "admin"
}

@Serializable
data class LoginRequest(
    val username: String,
    val password: String
)

@Serializable
data class LoginResponse(
    val user: User,
    val token: String,
    @SerialName("refresh_token")
    val refreshToken: String,
    @SerialName("expires_in")
    val expiresIn: Long
)

@Serializable
data class RegisterRequest(
    val username: String,
    val email: String,
    val password: String,
    @SerialName("first_name")
    val firstName: String,
    @SerialName("last_name")
    val lastName: String
)

@Serializable
data class AuthStatus(
    val authenticated: Boolean,
    val user: User? = null,
    val permissions: List<String>? = null,
    val error: String? = null
)

/**
 * UI state holder for authentication status, loading indicator, and error messages.
 */
data class AuthState(
    val isAuthenticated: Boolean = false,
    val isLoading: Boolean = false,
    val error: String? = null,
    val user: User? = null
)

@Serializable
data class ChangePasswordRequest(
    @SerialName("current_password")
    val currentPassword: String,
    @SerialName("new_password")
    val newPassword: String
)

@Serializable
data class UpdateProfileRequest(
    @SerialName("first_name")
    val firstName: String? = null,
    @SerialName("last_name")
    val lastName: String? = null,
    val email: String? = null
)

@Serializable
data class ApiResponse<T>(
    val data: T? = null,
    val error: String? = null,
    val message: String? = null
)

@Serializable
data class ErrorResponse(
    val error: String,
    val code: Int? = null,
    val details: Map<String, String>? = null
)

/**
 * Enumeration of user roles with their API values and display names.
 */
enum class UserRole(val value: String, val displayName: String) {
    ADMIN("admin", "Administrator"),
    MODERATOR("moderator", "Moderator"),
    USER("user", "User"),
    VIEWER("viewer", "Viewer");

    companion object {
        fun fromValue(value: String): UserRole {
            return values().find { it.value == value } ?: USER
        }
    }
}

/**
 * Constants for permission strings used in role-based access control.
 */
object Permissions {
    // Media permissions
    const val READ_MEDIA = "read:media"
    const val WRITE_MEDIA = "write:media"
    const val DELETE_MEDIA = "delete:media"

    // Catalog permissions
    const val READ_CATALOG = "read:catalog"
    const val WRITE_CATALOG = "write:catalog"
    const val DELETE_CATALOG = "delete:catalog"

    // Analysis permissions
    const val TRIGGER_ANALYSIS = "trigger:analysis"
    const val VIEW_ANALYSIS = "view:analysis"

    // Admin permissions
    const val MANAGE_USERS = "manage:users"
    const val MANAGE_ROLES = "manage:roles"
    const val VIEW_LOGS = "view:logs"
    const val SYSTEM_ADMIN = "admin:system"

    // API permissions
    const val API_ACCESS = "access:api"
    const val API_WRITE = "write:api"
}
/** Response of POST /api/v1/ws/ticket (SEC-WS-1 F2): a single-use ticket valid for [expiresIn] seconds. */
@Serializable
data class WsTicketResponse(
    val ticket: String,
    @kotlinx.serialization.SerialName("expires_at") val expiresAt: String? = null,
    @kotlinx.serialization.SerialName("expires_in") val expiresIn: Int? = null
)
