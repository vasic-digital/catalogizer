// Package challenges provides challenge registration for
// the Catalogizer system. Import this package to register
// all built-in challenges with the provided service.
package challenges

import (
	"catalogizer/services"
	"fmt"
	"os"
)

// RegisterAll registers all built-in Catalogizer challenges
// with the given challenge service. If the endpoint config
// file is missing, registration is silently skipped (common
// in dev environments without NAS access).
func RegisterAll(svc *services.ChallengeService) error {
	// NAS endpoint challenges (only if config exists)
	cfg, err := LoadEndpointConfig(DefaultConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("challenges: config not loaded: %v\n", err)
		}
	} else {
		for _, ep := range cfg.Endpoints {
			endpoint := ep // capture loop variable

			svc.Register(NewSMBConnectivityChallenge(&endpoint))
			svc.Register(NewDirectoryDiscoveryChallenge(&endpoint))

			for _, dir := range endpoint.Directories {
				d := dir // capture loop variable
				switch d.ContentType {
				case "music":
					svc.Register(NewMusicScanChallenge(&endpoint, d))
				case "tv_show":
					svc.Register(NewSeriesScanChallenge(&endpoint, d))
				case "movie":
					svc.Register(NewMoviesScanChallenge(&endpoint, d))
				case "software":
					svc.Register(NewSoftwareScanChallenge(&endpoint, d))
				case "comic":
					svc.Register(NewComicsScanChallenge(&endpoint, d))
				}
			}
		}
	}

	// All challenges below register regardless of NAS endpoint config

	// Populate challenge (triggers full scan pipeline via API)
	svc.Register(NewFirstCatalogPopulateChallenge())

	// Browsing challenges (validate the running API and web app)
	svc.Register(NewBrowsingAPIHealthChallenge())
	svc.Register(NewBrowsingAPICatalogChallenge())
	svc.Register(NewBrowsingWebAppChallenge())

	// Asset challenges (validate asset lazy loading pipeline)
	svc.Register(NewAssetServingChallenge())
	svc.Register(NewAssetLazyLoadingChallenge())

	// Database challenges (validate database connectivity and schema)
	svc.Register(NewDatabaseConnectivityChallenge())
	svc.Register(NewDatabaseSchemaValidationChallenge())

	// Entity challenges (validate entity system after scan)
	svc.Register(NewEntityAggregationChallenge())
	svc.Register(NewEntityBrowsingChallenge())
	svc.Register(NewEntityMetadataChallenge())
	svc.Register(NewEntityDuplicatesChallenge())
	svc.Register(NewEntityHierarchyChallenge())

	// Module integration challenges (CH-021 to CH-025)
	// Validate the API endpoints used by vasic-digital TypeScript modules
	svc.Register(NewCollectionsAPIChallenge())     // CH-021: @vasic-digital/collection-manager
	svc.Register(NewEntityUserMetadataChallenge()) // CH-022: @vasic-digital/media-browser
	svc.Register(NewEntitySearchChallenge())       // CH-023: @vasic-digital/media-browser
	svc.Register(NewStorageRootsAPIChallenge())    // CH-024: @vasic-digital/catalogizer-api-client
	svc.Register(NewAuthTokenRefreshChallenge())   // CH-025: @vasic-digital/catalogizer-api-client

	// Extended validation challenges (CH-026 to CH-035)
	svc.Register(NewStressTestChallenge())           // CH-026: API stress test
	svc.Register(NewRateLimitingChallenge())         // CH-027: Rate limiting
	svc.Register(NewFavoritesWorkflowChallenge())    // CH-028: Favorites workflow
	svc.Register(NewCollectionManagementChallenge()) // CH-029: Collection management
	svc.Register(NewMediaPlaybackChallenge())        // CH-030: Media playback
	svc.Register(NewSearchFilterChallenge())         // CH-031: Search & filter
	svc.Register(NewCoverArtChallenge())             // CH-032: Cover art
	svc.Register(NewWebSocketEventsChallenge())      // CH-033: WebSocket events

	// Image quality challenges (CH-IQ-*)
	RegisterImageQualityChallenges(svc)
	RegisterExtendedImageQualityChallenges(svc)
	RegisterFinalImageQualityChallenges(svc)

	// Nexus-authored Catalogizer challenges (CH-NX-WEBFLOW-*, ...)
	RegisterNexusChallenges(svc)
	svc.Register(NewSecurityChallenge())             // CH-034: Security
	svc.Register(NewConfigWizardChallenge())         // CH-035: Configuration wizard

	// Security validation challenges (CH-036 to CH-040)
	svc.Register(NewAuthRequiredChallenge())      // CH-036: Auth required on endpoints
	svc.Register(NewJWTExpirationChallenge())     // CH-037: JWT expiration enforced
	svc.Register(NewRateLimitAuthChallenge())     // CH-038: Rate limiting on auth
	svc.Register(NewCORSHeadersChallenge())       // CH-039: CORS headers
	svc.Register(NewNoSensitiveErrorsChallenge()) // CH-040: No sensitive data in errors

	// Performance regression challenges (CH-041 to CH-044)
	svc.Register(NewHealthLatencyChallenge())       // CH-041: Health < 10ms
	svc.Register(NewFileListingLatencyChallenge())  // CH-042: File listing < 200ms
	svc.Register(NewEntitySearchLatencyChallenge()) // CH-043: Entity search < 500ms
	svc.Register(NewWebSocketLatencyChallenge())    // CH-044: WebSocket < 50ms

	// Documentation completeness challenges (CH-045 to CH-047)
	svc.Register(NewAPIDocsChallenge())      // CH-045: API docs
	svc.Register(NewDatabaseDocsChallenge()) // CH-046: Database docs
	svc.Register(NewConfigDocsChallenge())   // CH-047: Config docs

	// System resilience challenges (CH-048 to CH-050)
	svc.Register(NewDBErrorRecoveryChallenge())  // CH-048: DB error recovery
	svc.Register(NewScannerRecoveryChallenge())  // CH-049: Scanner recovery
	svc.Register(NewGracefulShutdownChallenge()) // CH-050: Graceful shutdown

	// Extended API validation challenges (CH-051 to CH-060)
	svc.Register(NewInputValidationChallenge())   // CH-051: Input validation & sanitization
	svc.Register(NewPaginationChallenge())        // CH-052: Pagination consistency
	svc.Register(NewContentTypesChallenge())      // CH-053: Content-Type validation
	svc.Register(NewUserManagementChallenge())    // CH-054: User management API
	svc.Register(NewAnalyticsAPIChallenge())      // CH-055: Analytics & statistics API
	svc.Register(NewEntityCRUDChallenge())        // CH-056: Entity CRUD operations
	svc.Register(NewSyncAPIChallenge())           // CH-057: Synchronization API
	svc.Register(NewSubtitleAPIChallenge())       // CH-058: Subtitle management API
	svc.Register(NewRecommendationAPIChallenge()) // CH-059: Recommendation engine API
	svc.Register(NewLocalizationAPIChallenge())   // CH-060: Localization & i18n API

	// Search and browse challenges (CH-061 to CH-065)
	svc.Register(NewSearchAPIBasicQueryChallenge())         // CH-061: Search API basic query
	svc.Register(NewSearchAPIDuplicateDetectionChallenge()) // CH-062: Search API duplicate detection
	svc.Register(NewSearchAPIAdvancedFiltersChallenge())    // CH-063: Search API advanced filters
	svc.Register(NewBrowseAPIStorageRootsChallenge())       // CH-064: Browse API storage roots
	svc.Register(NewBrowseAPIDirectoryListingChallenge())   // CH-065: Browse API directory listing

	// Sync and security challenges (CH-066 to CH-070)
	svc.Register(NewSyncAPIEndpointCreationChallenge()) // CH-066: Sync API endpoint creation
	svc.Register(NewSyncAPICloudProvidersChallenge())   // CH-067: Sync API cloud providers
	svc.Register(NewSyncAPIUserEndpointsChallenge())    // CH-068: Sync API user endpoints
	svc.Register(NewSecurityHeadersAllChallenge())      // CH-069: Security headers present
	svc.Register(NewCORSRejectsUnauthorizedChallenge()) // CH-070: CORS rejects unauthorized origins

	// Security validation challenges (CH-071 to CH-075)
	svc.Register(NewInputValidationRejectsInjectionChallenge()) // CH-071: Input validation rejects injection
	svc.Register(NewRateLimitAuthEndpointsChallenge())          // CH-072: Rate limit auth endpoints
	svc.Register(NewJWTTokenLifecycleChallenge())               // CH-073: JWT token lifecycle
	svc.Register(NewFileUploadMagicBytesChallenge())            // CH-074: File upload magic bytes
	svc.Register(NewConversionRejectsPathTraversalChallenge())  // CH-075: Path traversal rejection

	// Performance challenges (CH-076 to CH-078)
	svc.Register(NewAPIResponseLatencyChallenge())    // CH-076: API response latency
	svc.Register(NewAPIConcurrentRequestsChallenge()) // CH-077: API concurrent requests
	svc.Register(NewGracefulDegradationChallenge())   // CH-078: Graceful degradation

	// Resilience challenges (CH-079 to CH-080)
	svc.Register(NewMemoryStableDuringLoadChallenge()) // CH-079: Memory stable during load
	svc.Register(NewDBPoolRecoveryChallenge())         // CH-080: DB pool recovery

	// WebSocket and runtime challenges (CH-081 to CH-083)
	svc.Register(NewWebSocketReconnectionChallenge())     // CH-081: WebSocket reconnection
	svc.Register(NewLazyInitOnFirstRequestChallenge())    // CH-082: Lazy init on first request
	svc.Register(NewSemaphorePreventsOverloadChallenge()) // CH-083: Semaphore prevents overload

	// Observability challenges (CH-084 to CH-088)
	svc.Register(NewPrometheusMetricsEndpointChallenge())   // CH-084: Prometheus metrics endpoint
	svc.Register(NewHTTPRequestMetricsIncrementChallenge()) // CH-085: HTTP request metrics increment
	svc.Register(NewRuntimeMetricsCurrentChallenge())       // CH-086: Runtime metrics current
	svc.Register(NewDBQueryDurationTrackedChallenge())      // CH-087: DB query duration tracked
	svc.Register(NewGrafanaDashboardRendersChallenge())     // CH-088: Grafana dashboard config exists

	// Performance verification challenges (CH-089 to CH-093)
	svc.Register(NewScanSemaphoreLimitsChallenge())       // CH-089: Scan semaphore limits
	svc.Register(NewRedisConnectionPoolingChallenge())    // CH-090: Redis connection pooling
	svc.Register(NewHTTPClientPooledTransportChallenge()) // CH-091: HTTP client pooled transport
	svc.Register(NewGoroutineLeakDetectionChallenge())    // CH-092: Goroutine leak detection
	svc.Register(NewHealthEndpointLatencyChallenge())     // CH-093: Health endpoint < 100ms

	// Provider verification challenges (CH-094 to CH-098)
	svc.Register(NewOpenLibraryProviderChallenge())         // CH-094: OpenLibrary provider search
	svc.Register(NewMusicBrainzProviderChallenge())         // CH-095: MusicBrainz provider search
	svc.Register(NewProviderGracefulDegradationChallenge()) // CH-096: Provider graceful degradation
	svc.Register(NewProviderManagerRoutingChallenge())      // CH-097: Provider manager routing
	svc.Register(NewLazyServiceRegistryChallenge())         // CH-098: LazyServiceRegistry init

	// User flow challenges (UF-*): exhaustive multi-platform
	// user flow automation across all 6 Catalogizer applications
	RegisterUserFlowAPIChallenges(svc)     // 49 API challenges
	RegisterUserFlowWebChallenges(svc)     // 59 web browser challenges
	RegisterUserFlowDesktopChallenges(svc) // 28 desktop + wizard challenges
	RegisterUserFlowMobileChallenges(svc)  // 38 Android + TV challenges

	// Module verification challenges (MOD-001 to MOD-015)
	// Verify each decoupled Go module has proper structure and docs
	RegisterModuleChallenges(svc)

	// Module functional verification challenges (MOD-016 to MOD-021)
	// Verify specific module capabilities (types, functions, patterns)
	RegisterModuleFuncChallenges(svc)

	// Admin API challenges (CH-100 to CH-110)
	RegisterAdminChallenges(svc)

	// Conversion API challenges (CH-111 to CH-120)
	RegisterConversionChallenges(svc)

	// Entity system challenges (CH-121 to CH-130)
	RegisterEntityChallenges(svc)

	// Middleware stack challenges (CH-131 to CH-140)
	RegisterMiddlewareChallenges(svc)

	// Phase 1 endpoint challenges (CH-141 to CH-152)
	RegisterPhase1Challenges(svc)

	// Module integration challenges (CH-153 to CH-158)
	RegisterModuleIntegrationChallenges(svc)

	// Playlist challenges (CH-159 to CH-170)
	RegisterPlaylistChallenges(svc)

	// Media analysis challenges (CH-171 to CH-185)
	RegisterMediaAnalysisChallenges(svc)

	// Security challenges (CH-186 to CH-200)
	RegisterSecurityChallenges(svc)

	// Performance challenges (CH-201 to CH-220)
	RegisterPerformanceChallenges(svc)

	// Multiplatform challenges (CH-221 to CH-250)
	RegisterMultiplatformChallenges(svc)

	// v2.1.0 expansion challenges (CH-251 to CH-401)
	RegisterConcurrencyHardeningChallenges(svc) // CH-251 to CH-255
	RegisterSecurityScanChallenges(svc)         // CH-256 to CH-260
	RegisterCoverageGateChallenges(svc)         // CH-261 to CH-270
	RegisterSubmoduleCoverageChallenges(svc)    // CH-271 to CH-278
	RegisterPlatformCoverageChallenges(svc)     // CH-279 to CH-283
	RegisterStressPerformanceChallenges(svc)    // CH-284 to CH-291
	RegisterDocumentationChallenges(svc)        // CH-352 to CH-364

	return nil
}

// RegisterAdminChallenges registers admin API challenges
// CH-100 to CH-110.
func RegisterAdminChallenges(svc *services.ChallengeService) {
	svc.Register(NewAdminSystemInfoChallenge())        // CH-100
	svc.Register(NewAdminUsersListChallenge())         // CH-101
	svc.Register(NewAdminUserUpdateChallenge())        // CH-102
	svc.Register(NewAdminStorageInfoChallenge())       // CH-103
	svc.Register(NewAdminBackupsListChallenge())       // CH-104
	svc.Register(NewAdminBackupCreateChallenge())      // CH-105
	svc.Register(NewAdminBackupRestoreChallenge())     // CH-106
	svc.Register(NewAdminStorageScanChallenge())       // CH-107
	svc.Register(NewAdminAuthRequiredChallenge())      // CH-108
	svc.Register(NewAdminNonAdminRejectedChallenge())  // CH-109
	svc.Register(NewAdminSystemInfoVersionChallenge()) // CH-110
}

// RegisterConversionChallenges registers conversion API
// challenges CH-111 to CH-120.
func RegisterConversionChallenges(svc *services.ChallengeService) {
	svc.Register(NewConversionJobsListChallenge())            // CH-111
	svc.Register(NewConversionJobCreateChallenge())           // CH-112
	svc.Register(NewConversionJobCancelChallenge())           // CH-113
	svc.Register(NewConversionJobRetryChallenge())            // CH-114
	svc.Register(NewConversionJobDownloadChallenge())         // CH-115
	svc.Register(NewConversionAuthRequiredChallenge())        // CH-116
	svc.Register(NewConversionJobStatusTransitionChallenge()) // CH-117
	svc.Register(NewConversionFormatsListChallenge())         // CH-118
	svc.Register(NewConversionInvalidJobIDChallenge())        // CH-119
	svc.Register(NewConversionRateLimitChallenge())           // CH-120
}

// RegisterEntityChallenges registers entity system challenges
// CH-121 to CH-130.
func RegisterEntityChallenges(svc *services.ChallengeService) {
	svc.Register(NewEntityListChallenge())               // CH-121
	svc.Register(NewEntityDetailChallenge())             // CH-122
	svc.Register(NewEntityTypesChallenge())              // CH-123
	svc.Register(NewEntityStatsChallenge())              // CH-124
	svc.Register(NewEntitySearchByTitleChallenge())      // CH-125
	svc.Register(NewEntityFilterByTypeChallenge())       // CH-126
	svc.Register(NewEntityHierarchyCorrectChallenge())   // CH-127
	svc.Register(NewEntityDuplicateDetectionChallenge()) // CH-128
	svc.Register(NewEntityPaginationChallenge())         // CH-129
	svc.Register(NewEntitySortingChallenge())            // CH-130
}

// RegisterMiddlewareChallenges registers middleware stack
// challenges CH-131 to CH-140.
func RegisterMiddlewareChallenges(svc *services.ChallengeService) {
	svc.Register(NewMiddlewareSecurityHeadersChallenge())       // CH-131
	svc.Register(NewMiddlewareRequestIDChallenge())             // CH-132
	svc.Register(NewMiddlewareCacheHeadersHealthChallenge())    // CH-133
	svc.Register(NewMiddlewareCacheHeadersDiscoveryChallenge()) // CH-134
	svc.Register(NewMiddlewareRateLimitingChallenge())          // CH-135
	svc.Register(NewMiddlewareRequestTimeoutChallenge())        // CH-136
	svc.Register(NewMiddlewareCORSHeadersChallenge())           // CH-137
	svc.Register(NewMiddlewareCompressionChallenge())           // CH-138
	svc.Register(NewMiddlewareSQLInjectionChallenge())          // CH-139
	svc.Register(NewMiddlewareXSSChallenge())                   // CH-140
}

// RegisterPhase1Challenges registers Phase 1 endpoint
// challenges CH-141 to CH-152.
func RegisterPhase1Challenges(svc *services.ChallengeService) {
	svc.Register(NewRecentMediaAPIChallenge())   // CH-141
	svc.Register(NewPopularMediaAPIChallenge())  // CH-142
	svc.Register(NewMediaByPathAPIChallenge())   // CH-143
	svc.Register(NewMediaAnalysisChallenge())    // CH-144
	svc.Register(NewMetadataRefreshChallenge())  // CH-145
	svc.Register(NewMediaQualityChallenge())     // CH-146
	svc.Register(NewChangePasswordChallenge())   // CH-147
	svc.Register(NewBackupCreateChallenge())     // CH-148
	svc.Register(NewBackupListChallenge())       // CH-149
	svc.Register(NewBackupRestoreChallenge())    // CH-150
	svc.Register(NewStorageScanAdminChallenge()) // CH-151
	svc.Register(NewBackupSemaphoreChallenge())  // CH-152
}

// RegisterModuleIntegrationChallenges registers module
// integration challenges CH-153 to CH-158.
func RegisterModuleIntegrationChallenges(svc *services.ChallengeService) {
	svc.Register(NewModuleRegistryInitChallenge())     // CH-153
	svc.Register(NewMemoryMonitorActiveChallenge())    // CH-154
	svc.Register(NewHealthAggregatorActiveChallenge()) // CH-155
	svc.Register(NewResilienceFacadeActiveChallenge()) // CH-156
	svc.Register(NewGuardrailEngineActiveChallenge())  // CH-157
	svc.Register(NewStorageResolverActiveChallenge())  // CH-158
}

// RegisterPlaylistChallenges registers playlist API challenges
// CH-159 to CH-170.
func RegisterPlaylistChallenges(svc *services.ChallengeService) {
	svc.Register(NewCreatePlaylistChallenge())        // CH-159
	svc.Register(NewListPlaylistsChallenge())         // CH-160
	svc.Register(NewGetPlaylistChallenge())           // CH-161
	svc.Register(NewUpdatePlaylistChallenge())        // CH-162
	svc.Register(NewDeletePlaylistChallenge())        // CH-163
	svc.Register(NewAddPlaylistItemChallenge())       // CH-164
	svc.Register(NewRemovePlaylistItemChallenge())    // CH-165
	svc.Register(NewPlaylistOrderingChallenge())      // CH-166
	svc.Register(NewPlaylistAccessControlChallenge()) // CH-167
	svc.Register(NewPublicPlaylistAccessChallenge())  // CH-168
	svc.Register(NewPlaylistPaginationChallenge())    // CH-169
	svc.Register(NewPlaylistSearchChallenge())        // CH-170
}

// RegisterMediaAnalysisChallenges registers media analysis
// challenges CH-171 to CH-185.
func RegisterMediaAnalysisChallenges(svc *services.ChallengeService) {
	svc.Register(NewMediaTypeDetectionChallenge())  // CH-171
	svc.Register(NewTVShowHierarchyChallenge())     // CH-172
	svc.Register(NewMusicHierarchyChallenge())      // CH-173
	svc.Register(NewDuplicateDetectionChallenge())  // CH-174
	svc.Register(NewEntitySearchAPIChallenge())     // CH-175
	svc.Register(NewEntityPaginationAPIChallenge()) // CH-176
	svc.Register(NewMediaItemCreateChallenge())     // CH-177
	svc.Register(NewMediaItemUpdateChallenge())     // CH-178
	svc.Register(NewMediaFileLinkChallenge())       // CH-179
	svc.Register(NewExternalMetadataChallenge())    // CH-180
	svc.Register(NewUserMetadataAPIChallenge())     // CH-181
	svc.Register(NewCollectionCreateAPIChallenge()) // CH-182
	svc.Register(NewCollectionItemsAPIChallenge())  // CH-183
	svc.Register(NewCollectionSearchAPIChallenge()) // CH-184
	svc.Register(NewDirectoryAnalysisChallenge())   // CH-185
}

// RegisterSecurityChallenges registers security validation
// challenges CH-186 to CH-200.
func RegisterSecurityChallenges(svc *services.ChallengeService) {
	svc.Register(NewJWTValidationChallenge())           // CH-186
	svc.Register(NewTokenRefreshSecurityChallenge())    // CH-187
	svc.Register(NewRoleBasedAccessSecurityChallenge()) // CH-188
	svc.Register(NewInputValidationSecurityChallenge()) // CH-189
	svc.Register(NewSQLInjectionSecurityChallenge())    // CH-190
	svc.Register(NewPathTraversalSecurityChallenge())   // CH-191
	svc.Register(NewRateLimitingSecurityChallenge())    // CH-192
	svc.Register(NewCORSHeadersSecurityChallenge())     // CH-193
	svc.Register(NewSecurityHeadersPresentChallenge())  // CH-194
	svc.Register(NewPasswordHashingChallenge())         // CH-195
	svc.Register(NewSessionTimeoutChallenge())          // CH-196
	svc.Register(NewBruteForceProtectionChallenge())    // CH-197
	svc.Register(NewContentTypeValidationChallenge())   // CH-198
	svc.Register(NewMaxBodySizeChallenge())             // CH-199
	svc.Register(NewErrorInfoLeakChallenge())           // CH-200
}

// RegisterPerformanceChallenges registers performance
// challenges CH-201 to CH-220.
func RegisterPerformanceChallenges(svc *services.ChallengeService) {
	svc.Register(NewHealthEndpointFastChallenge())    // CH-201
	svc.Register(NewSearchLatencyChallenge())         // CH-202
	svc.Register(NewBrowseLatencyChallenge())         // CH-203
	svc.Register(NewEntityDetailLatencyChallenge())   // CH-204
	svc.Register(NewStatsLatencyChallenge())          // CH-205
	svc.Register(NewConcurrentReadsChallenge())       // CH-206
	svc.Register(NewConcurrentSearchesChallenge())    // CH-207
	svc.Register(NewGoroutineStabilityChallenge())    // CH-208
	svc.Register(NewMemoryStabilityChallenge())       // CH-209
	svc.Register(NewConnectionPoolHealthChallenge())  // CH-210
	svc.Register(NewCacheEffectivenessChallenge())    // CH-211
	svc.Register(NewCompressionActiveChallenge())     // CH-212
	svc.Register(NewMetricsEndpointPerfChallenge())   // CH-213
	svc.Register(NewDeepHealthCheckChallenge())       // CH-214
	svc.Register(NewWebSocketConnectPerfChallenge())  // CH-215
	svc.Register(NewWebSocketMessageChallenge())      // CH-216
	svc.Register(NewScanQueueingChallenge())          // CH-217
	svc.Register(NewBackupPerformanceChallenge())     // CH-218
	svc.Register(NewPaginationPerformanceChallenge()) // CH-219
	svc.Register(NewFilterPerformanceChallenge())     // CH-220
}

// RegisterMultiplatformChallenges registers API consistency,
// data integrity, and admin operations challenges CH-221
// to CH-250.
func RegisterMultiplatformChallenges(svc *services.ChallengeService) {
	// API consistency (CH-221 to CH-230)
	svc.Register(NewConsistentJSONStructureChallenge())   // CH-221
	svc.Register(NewConsistentErrorFormatChallenge())     // CH-222
	svc.Register(NewConsistentPaginationChallenge())      // CH-223
	svc.Register(NewConsistentStatusCodesChallenge())     // CH-224
	svc.Register(NewConsistentContentTypeChallenge())     // CH-225
	svc.Register(NewConsistentAuthRequiredChallenge())    // CH-226
	svc.Register(NewConsistentTimestampFormatChallenge()) // CH-227
	svc.Register(NewConsistentIDFormatChallenge())        // CH-228
	svc.Register(NewConsistentMethodSupportChallenge())   // CH-229
	svc.Register(NewConsistentEmptyResponseChallenge())   // CH-230

	// Data integrity (CH-231 to CH-240)
	svc.Register(NewEntityRelationshipsValidChallenge()) // CH-231
	svc.Register(NewNoOrphanMediaFilesChallenge())       // CH-232
	svc.Register(NewEntityTypeConsistencyChallenge())    // CH-233
	svc.Register(NewCollectionIntegrityChallenge())      // CH-234
	svc.Register(NewStorageRootIntegrityChallenge())     // CH-235
	svc.Register(NewScanHistoryIntegrityChallenge())     // CH-236
	svc.Register(NewUserDataIntegrityChallenge())        // CH-237
	svc.Register(NewFavoritesIntegrityChallenge())       // CH-238
	svc.Register(NewMediaStatsIntegrityChallenge())      // CH-239
	svc.Register(NewDetectionRulesIntegrityChallenge())  // CH-240

	// Admin operations (CH-241 to CH-250)
	svc.Register(NewAdminSystemInfoAccessChallenge()) // CH-241
	svc.Register(NewAdminUserListAccessChallenge())   // CH-242
	svc.Register(NewAdminStorageOverviewChallenge())  // CH-243
	svc.Register(NewAdminLogCollectionChallenge())    // CH-244
	svc.Register(NewAdminErrorReportingChallenge())   // CH-245
	svc.Register(NewAdminBackupListChallenge())       // CH-246
	svc.Register(NewAdminUserUpdateOpsChallenge())    // CH-247
	svc.Register(NewAdminChallengesListChallenge())   // CH-248
	svc.Register(NewAdminConfigAccessChallenge())     // CH-249
	svc.Register(NewAdminHealthDashboardChallenge())  // CH-250

	// Playback session tracking (CH-200 reserved in the plan,
	// registered here alongside the admin API suites).
	svc.Register(NewPlaybackSessionsAPIChallenge()) // CH-200: Playback sessions lifecycle
}
