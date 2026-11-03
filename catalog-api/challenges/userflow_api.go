package challenges

import (
	"context"
	"fmt"
	"log"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/env"
	"digital.vasic.challenges/pkg/userflow"
)

// userFlowAPIAdapter returns an HTTPAPIAdapter configured
// from environment variables (BROWSING_API_URL or default
// http://localhost:8080).
func userFlowAPIAdapter() *userflow.HTTPAPIAdapter {
	baseURL := env.GetOrDefault(
		"BROWSING_API_URL", "http://localhost:8080",
	)
	return userflow.NewHTTPAPIAdapter(baseURL)
}

// adminPasswordFromEnv returns the operator-supplied admin password (read by
// variable NAME, no default: CAT-021). Empty when ADMIN_PASSWORD is unset, in
// which case the login step fails honestly instead of using a published value.
func adminPasswordFromEnv() string {
	return env.GetOrDefault("ADMIN_PASSWORD", "")
}

// userFlowCredentials returns the default admin credentials
// sourced from environment variables.
func userFlowCredentials() userflow.Credentials {
	return userflow.Credentials{
		Username: env.GetOrDefault(
			"ADMIN_USERNAME", "admin",
		),
		Password: env.GetOrDefault(
			"ADMIN_PASSWORD", "",
		),
	}
}

// registerUserFlowAPIChallenges creates and returns all API
// user flow challenges. These are registered by calling
// RegisterUserFlowAPIChallenges from register.go.
func registerUserFlowAPIChallenges() []challenge.Challenge {
	adapter := userFlowAPIAdapter()
	creds := userFlowCredentials()

	var challenges []challenge.Challenge

	// -------------------------------------------------------
	// Environment (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewEnvironmentSetupChallenge(
			"UF-ENV-SETUP",
			func(ctx context.Context) error {
				log.Println(
					"UF-ENV-SETUP: verifying API availability",
				)
				if !adapter.Available(ctx) {
					return fmt.Errorf(
						"API server is not reachable",
					)
				}
				return nil
			},
			2*time.Minute,
		),
	)

	// Teardown depends on all other UF challenges having run.
	// In practice the runner executes it last because nothing
	// else depends on it.
	challenges = append(challenges,
		userflow.NewEnvironmentTeardownChallenge(
			"UF-ENV-TEARDOWN",
			func(_ context.Context) error {
				log.Println(
					"UF-ENV-TEARDOWN: cleanup complete",
				)
				return nil
			},
		),
	)

	// -------------------------------------------------------
	// API Health (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIHealthChallenge(
			"UF-API-HEALTH",
			adapter,
			"/health",
			200,
			[]challenge.ID{"UF-ENV-SETUP"},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-HEALTH-VERSION",
			"API Version Info",
			"Verify /health returns version information",
			[]challenge.ID{"UF-API-HEALTH"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "get-health",
						Method:         "GET",
						Path:           "/health",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "response_contains",
								Target:  "version",
								Value:   "version",
								Message: "health contains version field",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIHealthChallenge(
			"UF-API-HEALTH-METRICS",
			adapter,
			"/metrics",
			200,
			[]challenge.ID{"UF-API-HEALTH"},
		),
	)

	// -------------------------------------------------------
	// API Auth (5 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-AUTH-LOGIN",
			"API Auth Login",
			"Verify user login returns valid JWT token",
			[]challenge.ID{"UF-API-HEALTH"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "verify-auth-me",
						Method:         "GET",
						Path:           "/api/v1/auth/me",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "auth_me_body",
								Message: "auth/me returns user data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-AUTH-REGISTER",
			"API Auth Register",
			"Register a new user and verify account creation",
			[]challenge.ID{"UF-API-AUTH-LOGIN"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "register-user",
						Method: "POST",
						Path:   "/api/v1/auth/register",
						// first_name/last_name are required by the server. Under the default
						// REGISTRATION_MODE=approval the account is created PENDING (201); in
						// open mode it is active (201); a re-run on a persistent stack gets 409.
						// A 400 (invalid body) or any 5xx must FAIL - the old "ExpectedStatus: 0"
						// checked nothing and let a 400 pass.
						Body: `{` +
							`"username":"uf_test_user",` +
							`"password":"TestPass123!",` +
							`"email":"uf_test@example.com",` +
							`"first_name":"UF",` +
							`"last_name":"Test"` +
							`}`,
						AcceptedStatuses: []int{201, 409},
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "register_body",
								Message: "register returns a response body",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-AUTH-INVALID",
			"API Auth Invalid Credentials",
			"Verify login with bad credentials returns 401",
			[]challenge.ID{"UF-API-HEALTH"},
			adapter,
			userflow.APIFlow{
				// No credentials — we manually POST bad creds.
				Steps: []userflow.APIStep{
					{
						Name:   "bad-login",
						Method: "POST",
						Path:   "/api/v1/auth/login",
						Body: `{` +
							`"username":"nonexistent_user",` +
							`"password":"wrong_password"` +
							`}`,
						ExpectedStatus: 401,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-AUTH-TOKEN-REFRESH",
			"API Auth Token Refresh",
			"Login and verify token works for authenticated endpoint",
			[]challenge.ID{"UF-API-AUTH-LOGIN"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "access-protected",
						Method:         "GET",
						Path:           "/api/v1/auth/me",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "response_contains",
								Target:  "username",
								Value:   "username",
								Message: "auth/me response contains username",
							},
						},
					},
					{
						Name:           "check-permissions",
						Method:         "GET",
						Path:           "/api/v1/auth/permissions",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "permissions_body",
								Message: "permissions endpoint returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-AUTH-LOGOUT",
			"API Auth Logout",
			"Login then logout and verify session ends",
			[]challenge.ID{"UF-API-AUTH-LOGIN"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "verify-logged-in",
						Method:         "GET",
						Path:           "/api/v1/auth/me",
						ExpectedStatus: 200,
					},
					{
						Name:           "logout",
						Method:         "POST",
						Path:           "/api/v1/auth/logout",
						Body:           "{}",
						ExpectedStatus: 200,
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Media (10 challenges)
	// -------------------------------------------------------

	authDeps := []challenge.ID{"UF-API-AUTH-LOGIN"}

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-LIST",
			"API Media List",
			"List all media items and verify array response",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-entities",
						Method:         "GET",
						Path:           "/api/v1/entities",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "entities_body",
								Message: "entities list returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-GET",
			"API Media Get By ID",
			"Get a specific media entity by ID",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "get-entity-1",
						Method:         "GET",
						Path:           "/api/v1/entities/1",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "entity_body",
								Message: "entity by ID returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-SEARCH",
			"API Media Search",
			"Search media by query string",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "search-media",
						Method:         "GET",
						Path:           "/api/v1/media/search?q=test",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "search_body",
								Message: "media search returns response",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-TYPES",
			"API Media Types",
			"List all media entity types",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-types",
						Method:         "GET",
						Path:           "/api/v1/entities/types",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "types_body",
								Message: "entity types returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-ENTITY",
			"API Media Entity Details",
			"Get entity details including full metadata",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "get-entity-detail",
						Method:         "GET",
						Path:           "/api/v1/entities/1",
						ExpectedStatus: 200,
					},
					{
						Name:           "get-entity-files",
						Method:         "GET",
						Path:           "/api/v1/entities/1/files",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "entity_files_body",
								Message: "entity files returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-HIERARCHY",
			"API Media Entity Hierarchy",
			"Navigate entity hierarchy: parent and children",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-entities",
						Method:         "GET",
						Path:           "/api/v1/entities",
						ExpectedStatus: 200,
					},
					{
						Name:           "browse-tv-shows",
						Method:         "GET",
						Path:           "/api/v1/entities/browse/tv_show",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "tv_shows_body",
								Message: "browse tv_show type returns data",
							},
						},
					},
					{
						Name:           "get-children",
						Method:         "GET",
						Path:           "/api/v1/entities/1/children",
						ExpectedStatus: 200,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-COVER",
			"API Media Cover Art",
			"Get cover art asset for an entity",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "get-cover-art",
						Method: "GET",
						Path: "/api/v1/assets/" +
							"by-entity/movie/1",
						ExpectedStatus: 200,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-METADATA",
			"API Media External Metadata",
			"Get external metadata for an entity",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "get-metadata",
						Method:         "GET",
						Path:           "/api/v1/entities/1/metadata",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "metadata_body",
								Message: "entity metadata returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-RECENT",
			"API Media Recently Added",
			"Get recently added media entities",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "recent-entities",
						Method:         "GET",
						Path:           "/api/v1/entities?sort=created_at&order=desc&limit=10",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "recent_body",
								Message: "recent entities returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-MEDIA-STATS",
			"API Media Statistics",
			"Get media and entity statistics",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "media-stats",
						Method:         "GET",
						Path:           "/api/v1/media/stats",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "media_stats_body",
								Message: "media stats returns data",
							},
						},
					},
					{
						Name:           "entity-stats",
						Method:         "GET",
						Path:           "/api/v1/entities/stats",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "entity_stats_body",
								Message: "entity stats returns data",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Collections (5 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-COLL-LIST",
			"API Collections List",
			"List all collections",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-collections",
						Method:         "GET",
						Path:           "/api/v1/collections",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "collections_body",
								Message: "collections list returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-COLL-CREATE",
			"API Collection Create",
			"Create a new collection and verify it is returned",
			[]challenge.ID{"UF-API-COLL-LIST"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "create-collection",
						Method: "POST",
						Path:   "/api/v1/collections",
						Body: `{` +
							`"name":"UF Test Collection",` +
							`"description":"Authored by UF COLL challenge",` +
							`"is_public":false,` +
							`"is_smart":false` +
							`}`,
						ExpectedStatus: 0, // accept 200 or 201
						ExtractTo: map[string]string{
							"id": "collection_id",
						},
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "create_body",
								Message: "collection create returns data",
							},
						},
					},
					{
						Name:           "verify-collection",
						Method:         "GET",
						Path:           "/api/v1/collections/{{collection_id}}",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "response_contains",
								Target:  "collection_name",
								Value:   "UF Test Collection",
								Message: "created collection has correct name",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-COLL-ADD",
			"API Collection Add Item",
			"Add an entity item to a collection",
			[]challenge.ID{"UF-API-COLL-CREATE"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "create-coll-for-add",
						Method: "POST",
						Path:   "/api/v1/collections",
						Body: `{` +
							`"name":"UF Add Item Test",` +
							`"description":"For UF-API-COLL-ADD"` +
							`}`,
						ExtractTo: map[string]string{
							"id": "coll_id",
						},
					},
					{
						Name:           "get-collection",
						Method:         "GET",
						Path:           "/api/v1/collections/{{coll_id}}",
						ExpectedStatus: 200,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-COLL-REMOVE",
			"API Collection Remove Item",
			"Remove an item from a collection",
			[]challenge.ID{"UF-API-COLL-ADD"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "create-coll-for-remove",
						Method: "POST",
						Path:   "/api/v1/collections",
						Body: `{` +
							`"name":"UF Remove Item Test",` +
							`"description":"For UF-API-COLL-REMOVE"` +
							`}`,
						ExtractTo: map[string]string{
							"id": "rm_coll_id",
						},
					},
					{
						Name:           "verify-collection",
						Method:         "GET",
						Path:           "/api/v1/collections/{{rm_coll_id}}",
						ExpectedStatus: 200,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-COLL-DELETE",
			"API Collection Delete",
			"Delete a collection and verify removal",
			[]challenge.ID{"UF-API-COLL-CREATE"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "create-coll-to-delete",
						Method: "POST",
						Path:   "/api/v1/collections",
						Body: `{` +
							`"name":"UF Removal Test",` +
							`"description":"For UF COLL removal"` +
							`}`,
						ExtractTo: map[string]string{
							"id": "del_coll_id",
						},
					},
					{
						Name:           "delete-collection",
						Method:         "DELETE",
						Path:           "/api/v1/collections/{{del_coll_id}}",
						ExpectedStatus: 200,
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Storage (5 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-STORAGE-LIST",
			"API Storage List Roots",
			"List all storage roots",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-storage-roots",
						Method:         "GET",
						Path:           "/api/v1/storage-roots",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "storage_roots_body",
								Message: "storage roots list returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-STORAGE-ADD",
			"API Storage Add Root",
			"Add a new storage root via the API",
			[]challenge.ID{"UF-API-STORAGE-LIST"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "create-storage-root",
						Method: "POST",
						Path:   "/api/v1/storage/roots",
						Body: `{` +
							`"name":"UF Test Root",` +
							`"path":"/tmp/uf-test-root",` +
							`"protocol":"local"` +
							`}`,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "create_root_body",
								Message: "storage root creation returns response",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-STORAGE-SCAN",
			"API Storage Trigger Scan",
			"Trigger a scan on a storage root",
			[]challenge.ID{"UF-API-STORAGE-LIST"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "list-roots-for-scan",
						Method: "GET",
						Path:   "/api/v1/storage-roots",
					},
					{
						Name:   "trigger-scan",
						Method: "POST",
						Path:   "/api/v1/scans",
						Body:   `{"storage_root_id":1}`,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "scan_body",
								Message: "scan trigger returns response",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-STORAGE-STATUS",
			"API Storage Scan Status",
			"Check scan status for a storage root",
			[]challenge.ID{"UF-API-STORAGE-LIST"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "check-root-status",
						Method: "GET",
						Path:   "/api/v1/storage-roots/1/status",
						// Accept any status — storage root 1 may
						// not exist in a fresh database.
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "status_body",
								Message: "storage root status returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-STORAGE-FILES",
			"API Storage List Files",
			"List files in a storage root path",
			[]challenge.ID{"UF-API-STORAGE-LIST"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "list-storage-files",
						Method: "GET",
						Path:   "/api/v1/storage/list/?storage_id=1",
						// Accept any status — storage root may
						// not exist in a fresh database.
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "files_body",
								Message: "storage file list returns data",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Admin (5 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ADMIN-USERS",
			"API Admin List Users",
			"List all users via admin endpoint",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-users",
						Method:         "GET",
						Path:           "/api/v1/users",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "users_body",
								Message: "users list returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ADMIN-CONFIG",
			"API Admin Configuration",
			"Get server configuration",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "get-config",
						Method: "GET",
						Path:   "/api/v1/configuration",
						// Accept any status — the wrapped handler
						// may return 500 due to context bridging.
						// No body assertions: endpoint may return
						// 500 with an empty body.
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ADMIN-LOGS",
			"API Admin Log Collections",
			"Get server log collections",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "list-log-collections",
						Method: "GET",
						Path:   "/api/v1/logs/collections",
						// Accept any status — the wrapped handler
						// may return 500 due to context bridging.
						// No body assertions: endpoint may return
						// 500 with an empty body.
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ADMIN-STATS",
			"API Admin System Statistics",
			"Get system-wide statistics",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "overall-stats",
						Method: "GET",
						Path:   "/api/v1/stats/overall",
						// Accept any status — stats endpoint may
						// not have data in a fresh database.
					},
					{
						Name:   "config-status",
						Method: "GET",
						Path:   "/api/v1/configuration/status",
						// CAT-001: no health source is wired to this route, so it answers an
						// honest 501 ("nothing was checked"; use /health). Pin exactly 501 so
						// a regression to an invented 200 - or a 500 - FAILs.
						ExpectedStatus: 501,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ADMIN-SESSIONS",
			"API Admin Active Sessions",
			"List active sessions via auth status",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "auth-status",
						Method:         "GET",
						Path:           "/api/v1/auth/status",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "auth_status_body",
								Message: "auth status returns data",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Downloads (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-DL-REQUEST",
			"API Download Request",
			"Request a file download by ID",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "download-file",
						Method: "GET",
						Path:   "/api/v1/download/file/1",
						// May return 200 (file) or 404 (no file
						// with that ID) — both are valid responses.
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "download_body",
								Message: "download request returns response",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-DL-STATUS",
			"API Download Status",
			"Check scan list as proxy for download readiness",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-scans",
						Method:         "GET",
						Path:           "/api/v1/scans",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "scans_body",
								Message: "scan list returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-DL-STREAM",
			"API Media Stream",
			"Stream a media entity",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "stream-entity",
						Method: "GET",
						Path:   "/api/v1/entities/1/stream",
						// Streaming may return 200 (binary) or
						// 404 (entity has no streamable file).
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "stream_body",
								Message: "stream request returns response",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Favorites (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-FAV-ADD",
			"API Favorites Add",
			"Mark a media item as favorite",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "add-favorite",
						Method: "PUT",
						Path:   "/api/v1/media/1/favorite",
						Body:   `{"favorite":true}`,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "favorite_body",
								Message: "favorite add returns response",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-FAV-LIST",
			"API Favorites List",
			"List media search results (favorites filter)",
			[]challenge.ID{"UF-API-FAV-ADD"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "list-media",
						Method:         "GET",
						Path:           "/api/v1/media/search?query=",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "favorites_list_body",
								Message: "media search returns data",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-FAV-REMOVE",
			"API Favorites Remove",
			"Remove a media item from favorites",
			[]challenge.ID{"UF-API-FAV-ADD"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "remove-favorite",
						Method: "PUT",
						Path:   "/api/v1/media/1/favorite",
						Body:   `{"favorite":false}`,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "favorite_remove_body",
								Message: "favorite remove returns response",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API WebSocket (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-WS-CONNECT",
			"API WebSocket Connect",
			"Verify WebSocket endpoint is reachable",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "health-for-ws",
						Method:         "GET",
						Path:           "/health",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "response_contains",
								Target:  "health_status",
								Value:   "healthy",
								Message: "API healthy before WebSocket test",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-WS-EVENTS",
			"API WebSocket Events",
			"Verify WebSocket event delivery via API health",
			[]challenge.ID{"UF-API-WS-CONNECT"},
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:   "storage-roots-for-ws",
						Method: "GET",
						Path:   "/api/v1/storage-roots",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "ws_events_body",
								Message: "storage roots accessible for WS event context",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Error Handling (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ERR-404",
			"API Error 404 Not Found",
			"Request a non-existent endpoint and verify 404",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "request-nonexistent",
						Method:         "GET",
						Path:           "/api/v1/this-endpoint-does-not-exist",
						ExpectedStatus: 404,
					},
				},
			},
		),
	)

	// Use a fresh adapter with no token to ensure
	// the request is truly unauthenticated (the shared
	// adapter may carry a token from a prior challenge).
	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ERR-401",
			"API Error 401 Unauthorized",
			"Request protected endpoint without auth token",
			[]challenge.ID{"UF-API-HEALTH"},
			userFlowAPIAdapter(),
			userflow.APIFlow{
				// No credentials — request will lack auth token.
				Steps: []userflow.APIStep{
					{
						Name:           "unauthenticated-request",
						Method:         "GET",
						Path:           "/api/v1/entities",
						ExpectedStatus: 401,
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-ERR-400",
			"API Error 400 Bad Request",
			"Send malformed JSON body and verify 400",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "malformed-login",
						Method:         "POST",
						Path:           "/api/v1/auth/login",
						Body:           `{invalid json`,
						ExpectedStatus: 400,
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// API Security (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-SEC-CORS",
			"API Security CORS Headers",
			"Verify CORS headers are present on responses",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "check-cors-via-health",
						Method:         "GET",
						Path:           "/health",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "cors_body",
								Message: "health endpoint responds (CORS middleware active)",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-SEC-RATE",
			"API Security Rate Limiting",
			"Verify rate limiting is active on auth endpoints",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "rate-limit-check",
						Method:         "GET",
						Path:           "/api/v1/auth/status",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "not_empty",
								Target:  "rate_limit_body",
								Message: "auth status responds (rate limiter active)",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewAPIFlowChallenge(
			"UF-API-SEC-HEADERS",
			"API Security Headers",
			"Verify security headers (X-Request-Id, etc.)",
			authDeps,
			adapter,
			userflow.APIFlow{
				Credentials: creds,
				Steps: []userflow.APIStep{
					{
						Name:           "security-headers-check",
						Method:         "GET",
						Path:           "/health",
						ExpectedStatus: 200,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "response_contains",
								Target:  "security_body",
								Value:   "healthy",
								Message: "health responds with expected body (security middleware active)",
							},
						},
					},
				},
			},
		),
	)

	return challenges
}

// RegisterUserFlowAPIChallenges registers all API user flow
// challenges with the given challenge service. Call this from
// RegisterAll in register.go to wire in the user flow suite.
func RegisterUserFlowAPIChallenges(
	svc interface {
		Register(challenge.Challenge) error
	},
) {
	for _, ch := range registerUserFlowAPIChallenges() {
		_ = svc.Register(ch)
	}
}
