
### api: by rule (run3 -> run4 open)
- go:S1192 CRITICAL CODE_SMELL: 338 -> 338
- go:S3776 CRITICAL CODE_SMELL: 310 -> 310
- go:S4144 MAJOR CODE_SMELL: 16 -> 16
- go:S1186 CRITICAL CODE_SMELL: 10 -> 0
- go:S107 MAJOR CODE_SMELL: 10 -> 10
- go:S100 MINOR CODE_SMELL: 6 -> 6
- go:S117 MINOR CODE_SMELL: 2 -> 2
- go:S1871 MAJOR CODE_SMELL: 2 -> 2
- go:S3923 MAJOR BUG: 2 -> 0
- docker:S7031 MINOR CODE_SMELL: 2 -> 2
- go:S1125 MINOR CODE_SMELL: 1 -> 1
NEW (0): 

#### api CRITICAL non-S3776/S1192: total 10 GONE 10 STILL 0
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:324
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:327
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:329
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:333
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:339
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:343
  GONE  go:S1186 CRITICAL catalog-api/internal/services/cover_art_service.go:617
  GONE  go:S1186 CRITICAL catalog-api/services/pdf_page_render.go:73
  GONE  go:S1186 CRITICAL catalog-api/services/conversion_proc_pdeathsig_other.go:9
  GONE  go:S1186 CRITICAL catalog-api/database/encryption.go:524

#### api CRITICAL all: total 658 GONE 10 STILL 648
  STILL go:S3776 CRITICAL catalog-api/internal/services/smb_ports_config_test.go:119
  STILL go:S3776 CRITICAL catalog-api/internal/services/smb_ports_config.go:140
  STILL go:S3776 CRITICAL catalog-api/internal/services/smb_globals_lint_test.go:24
  STILL go:S3776 CRITICAL catalog-api/smb/subroot.go:30
  STILL go:S3776 CRITICAL catalog-api/filesystem/settings.go:175
  STILL go:S3776 CRITICAL catalog-api/trusted_proxies_auto_test.go:241
  STILL go:S3776 CRITICAL catalog-api/trusted_proxies_auto_test.go:153
  STILL go:S3776 CRITICAL catalog-api/filesystem/s3_client.go:465
  STILL go:S3776 CRITICAL catalog-api/filesystem/s3_client.go:545
  STILL go:S3776 CRITICAL catalog-api/filesystem/s3_unit_test.go:84
  STILL go:S3776 CRITICAL catalog-api/filesystem/s3_unit_test.go:220
  STILL go:S3776 CRITICAL catalog-api/main.go:267
  STILL go:S3776 CRITICAL catalog-api/handlers/collection_handler.go:360
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_webdav_wdf1_chaos_test.go:178
  STILL go:S3776 CRITICAL catalog-api/services/webdav_client.go:242
  STILL go:S3776 CRITICAL catalog-api/services/playlist_service.go:65
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_webdav_wdf1_chaos_test.go:66
  STILL go:S3776 CRITICAL catalog-api/internal/netguard/operator.go:51
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_webdav_wdf1_probe_test.go:33
  STILL go:S3776 CRITICAL catalog-api/services/document_text_indexer.go:89
  STILL go:S3776 CRITICAL catalog-api/internal/services/subtitle_sidecar.go:157
  STILL go:S1192 CRITICAL catalog-api/internal/services/universal_scanner.go:452
  STILL go:S3776 CRITICAL catalog-api/handlers/admin_handler.go:245
  STILL go:S3776 CRITICAL catalog-api/handlers/admin_last_admin_race_d9_test.go:54
  STILL go:S3776 CRITICAL catalog-api/handlers/user_handler.go:247
  STILL go:S3776 CRITICAL catalog-api/handlers/pdf_pages_d3_test.go:348
  STILL go:S3776 CRITICAL catalog-api/internal/services/subtitle_sync.go:392
  STILL go:S3776 CRITICAL catalog-api/internal/services/subtitle_sync.go:449
  STILL go:S3776 CRITICAL catalog-api/main_subtitle_wiring_test.go:19
  STILL go:S3776 CRITICAL catalog-api/services/conversion_service.go:444
  STILL go:S3776 CRITICAL catalog-api/handlers/admin_handler.go:157
  STILL go:S1192 CRITICAL catalog-api/handlers/admin_handler.go:172
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_review_r3_unit_test.go:162
  STILL go:S3776 CRITICAL catalog-api/internal/services/subtitle_sync.go:243
  STILL go:S3776 CRITICAL catalog-api/services/conversion_service.go:1191
  STILL go:S1192 CRITICAL catalog-api/services/conversion_service.go:1449
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_review_r1_unit_test.go:22
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_realserver_live_test.go:97
  STILL go:S3776 CRITICAL catalog-api/internal/liveevidence/interp_options_test.go:251
  STILL go:S3776 CRITICAL catalog-api/internal/nilslice_guard/guard_test.go:69
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:736
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:1100
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:294
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:911
  STILL go:S3776 CRITICAL catalog-api/internal/liveevidence/liveevidence.go:395
  STILL go:S3776 CRITICAL catalog-api/internal/liveevidence/interp_options_test.go:77
  STILL go:S3776 CRITICAL catalog-api/internal/liveevidence/liveevidence.go:318
  STILL go:S3776 CRITICAL catalog-api/middleware/csrf.go:132
  STILL go:S3776 CRITICAL catalog-api/internal/liveevidence/liveevidence.go:244
  STILL go:S3776 CRITICAL catalog-api/internal/services/cover_art_service.go:507
  STILL go:S3776 CRITICAL catalog-api/middleware/auth.go:109
  STILL go:S3776 CRITICAL catalog-api/stream_ticket_binary_test.go:40
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:491
  STILL go:S3776 CRITICAL catalog-api/internal/netguard/netguard.go:116
  STILL go:S3776 CRITICAL catalog-api/challenges/http_challenges_execute_test.go:19
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:181
  STILL go:S3776 CRITICAL catalog-api/handlers/search.go:57
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:118
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:373
  STILL go:S3776 CRITICAL catalog-api/api_contract_live_test.go:984
  STILL go:S1192 CRITICAL catalog-api/handlers/log_management_handler.go:77
  STILL go:S1192 CRITICAL catalog-api/handlers/media_entity_handler.go:266
  STILL go:S3776 CRITICAL catalog-api/route_enumeration_probe_test.go:27
  STILL go:S3776 CRITICAL catalog-api/internal/services/live_webdav_scan_test.go:82
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_webdav_yandex_test.go:432
  STILL go:S3776 CRITICAL catalog-api/services/converter_hardening.go:337
  STILL go:S1192 CRITICAL catalog-api/handlers/playlist_handler.go:136
  STILL go:S3776 CRITICAL catalog-api/internal/smartrules/smartrules.go:226
  STILL go:S3776 CRITICAL catalog-api/internal/smartrules/smartrules.go:371
  STILL go:S1192 CRITICAL catalog-api/internal/smartrules/smartrules.go:730
  STILL go:S3776 CRITICAL catalog-api/services/playlist_service.go:189
  STILL go:S3776 CRITICAL catalog-api/smart_collections_binary_test.go:108
  STILL go:S3776 CRITICAL catalog-api/services/sync_service.go:106
  STILL go:S1192 CRITICAL catalog-api/main.go:1497
  STILL go:S1192 CRITICAL catalog-api/handlers/collection_handler.go:105
  STILL go:S3776 CRITICAL catalog-api/handlers/collection_handler.go:461
  STILL go:S1192 CRITICAL catalog-api/handlers/playlist_handler.go:192
  STILL go:S1192 CRITICAL catalog-api/internal/smartrules/smartrules.go:638
  STILL go:S1192 CRITICAL catalog-api/internal/smartrules/smartrules.go:916
  STILL go:S1192 CRITICAL catalog-api/main.go:1644
  STILL go:S3776 CRITICAL catalog-api/services/auth_service.go:97
  STILL go:S3776 CRITICAL catalog-api/main.go:2104
  STILL go:S1192 CRITICAL catalog-api/middleware/auth.go:225
  STILL go:S3776 CRITICAL catalog-api/scaling_soak_live_test.go:126
  STILL go:S3776 CRITICAL catalog-api/internal/media/realtime/watcher.go:543
  STILL go:S3776 CRITICAL catalog-api/internal/poolguard/scan.go:139
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:847
  STILL go:S3776 CRITICAL catalog-api/internal/services/catalog.go:410
  STILL go:S3776 CRITICAL catalog-api/repository/duplicate_entity_repository.go:29
  STILL go:S3776 CRITICAL catalog-api/scaling_live_test.go:741
  STILL go:S1192 CRITICAL catalog-api/middleware/auth.go:282
  STILL go:S3776 CRITICAL catalog-api/scaling_live_test.go:430
  STILL go:S3776 CRITICAL catalog-api/scaling_live_test.go:576
  STILL go:S3776 CRITICAL catalog-api/models/user.go:114
  STILL go:S3776 CRITICAL catalog-api/repository/conversion_repository.go:194
  STILL go:S3776 CRITICAL catalog-api/services/conversion_service.go:261
  STILL go:S3776 CRITICAL catalog-api/utils/validation.go:372
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:70
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:75
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:168
  STILL go:S1192 CRITICAL catalog-api/middleware/auth.go:233
  STILL go:S3776 CRITICAL catalog-api/handlers/subtitle_handler.go:169
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_webdav_test.go:101
  STILL go:S1192 CRITICAL catalog-api/handlers/conversion_handler.go:59
  STILL go:S3776 CRITICAL catalog-api/main_adminreset.go:51
  STILL go:S3776 CRITICAL catalog-api/services/pdf_text_isolation.go:311
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:225
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:226
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:229
  STILL go:S1192 CRITICAL catalog-api/middleware/auth.go:167
  STILL go:S3776 CRITICAL catalog-api/handlers/gin_adapter_muxvars_test.go:216
  STILL go:S3776 CRITICAL catalog-api/handlers/gin_adapter_muxvars_test.go:261
  STILL go:S3776 CRITICAL catalog-api/config/doc_placeholder_census_test.go:36
  STILL go:S3776 CRITICAL catalog-api/handlers/gin_adapter_muxvars_test.go:428
  STILL go:S3776 CRITICAL catalog-api/services/pdf_text_extractor.go:92
  STILL go:S1192 CRITICAL catalog-api/services/reporting_service.go:451
  STILL go:S3776 CRITICAL catalog-api/config/config.go:229
  STILL go:S3776 CRITICAL catalog-api/tests/security/injection_test.go:60
  STILL go:S3776 CRITICAL catalog-api/internal/services/recommendation_service.go:1092
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_nas_test.go:302
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_nas_test.go:370
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_nas_test.go:704
  STILL go:S3776 CRITICAL catalog-api/middleware/redis_rate_limiter.go:115
  STILL go:S1192 CRITICAL catalog-api/middleware/redis_rate_limiter.go:174
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_cancel_cleanup_unit_test.go:102
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_session_race_unit_test.go:231
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_session_race_unit_test.go:297
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_session_race_unit_test.go:676
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_session_race_unit_test.go:812
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_session_race_unit_test.go:1182
  STILL go:S3776 CRITICAL catalog-api/internal/services/generic_scanner.go:639
  STILL go:S3776 CRITICAL catalog-api/internal/pathguard/pathguard.go:82
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/copy.go:71
  STILL go:S3776 CRITICAL catalog-api/internal/pathguard/confine_extended_differential_test.go:101
  STILL go:S3776 CRITICAL catalog-api/internal/pathguard/confine_file_test.go:91
  STILL go:S3776 CRITICAL catalog-api/internal/pathguard/confine_differential_test.go:32
  STILL go:S3776 CRITICAL catalog-api/services/sync_service.go:1193
  STILL go:S3776 CRITICAL catalog-api/internal/pathguard/pathguard.go:198
  STILL go:S3776 CRITICAL catalog-api/internal/pathguard/zz_review2_test.go:13
  STILL go:S3776 CRITICAL catalog-api/database/encryption.go:793
  STILL go:S3776 CRITICAL catalog-api/filesystem/ftp_disconnect_race_unit_test.go:29
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_nas_test.go:164
  STILL go:S3776 CRITICAL catalog-api/filesystem/live_nas_test.go:642
  STILL go:S3776 CRITICAL catalog-api/database/connection.go:27
  STILL go:S3776 CRITICAL catalog-api/filesystem/settings.go:331
  STILL go:S3776 CRITICAL catalog-api/filesystem/wf22_r3_test.go:114
  STILL go:S3776 CRITICAL catalog-api/internal/services/wf22_r3_test.go:289
  STILL go:S3776 CRITICAL catalog-api/internal/services/wf22_review_ftp_probe2_test.go:25
  STILL go:S3776 CRITICAL catalog-api/filesystem/webdav_propfind.go:134
  STILL go:S3776 CRITICAL catalog-api/internal/handlers/eventbus_bridge.go:41
  STILL go:S3776 CRITICAL catalog-api/internal/services/generic_scanner.go:416
  STILL go:S3776 CRITICAL catalog-api/internal/services/universal_scanner.go:903
  STILL go:S3776 CRITICAL catalog-api/internal/services/wf22_createclient_gate_test.go:32
  STILL go:S3776 CRITICAL catalog-api/internal/services/wf22_createclient_gate_test.go:132
  STILL go:S3776 CRITICAL catalog-api/internal/services/wf22_review_ftp_probe_test.go:21
  STILL go:S3776 CRITICAL catalog-api/tests/realfs/generic_scanner_realfs_test.go:492
  STILL go:S3776 CRITICAL catalog-api/internal/infra/provisioner.go:270
  STILL go:S3776 CRITICAL catalog-api/internal/infra/provisioner_test.go:117
  STILL go:S1192 CRITICAL catalog-api/handlers/comic_pages_handler.go:434
  STILL go:S1192 CRITICAL catalog-api/handlers/comic_pages_handler.go:522
  STILL go:S3776 CRITICAL catalog-api/handlers/comic_pages_handler.go:377
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:1093
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:986
  STILL go:S3776 CRITICAL catalog-api/internal/services/smb_probe_real_test.go:78
  STILL go:S3776 CRITICAL catalog-api/internal/services/smb_discovery_real_test.go:29
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:1205
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:282
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:476
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:575
  STILL go:S3776 CRITICAL catalog-api/internal/services/universal_scanner.go:749
  STILL go:S3776 CRITICAL catalog-api/tests/stress/concurrent_handlers_test.go:155
  STILL go:S3776 CRITICAL catalog-api/tests/stress/load_test_context_helper_test.go:50
  STILL go:S3776 CRITICAL catalog-api/challenges/browsing_test.go:581
  STILL go:S1192 CRITICAL catalog-api/services/reporting_service.go:345
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:401
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:102
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:107
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:113
  STILL go:S1192 CRITICAL catalog-api/main.go:1101
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:716
  STILL go:S1192 CRITICAL catalog-api/challenges/image_quality_final.go:35
  STILL go:S1192 CRITICAL catalog-api/challenges/image_quality_extended.go:110
  STILL go:S1192 CRITICAL catalog-api/challenges/image_quality.go:70
  STILL go:S1192 CRITICAL catalog-api/challenges/image_quality.go:116
  STILL go:S1192 CRITICAL catalog-api/handlers/cover_handler.go:41
  STILL go:S1192 CRITICAL catalog-api/internal/services/cover_art_service.go:14
  STILL go:S1192 CRITICAL catalog-api/handlers/cover_handler.go:69
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:628
  STILL go:S3776 CRITICAL catalog-api/internal/media/providers/providers.go:176
  STILL go:S3776 CRITICAL catalog-api/main.go:2070
  STILL go:S3776 CRITICAL catalog-api/internal/services/cover_art_fallback_test.go:8
  STILL go:S3776 CRITICAL catalog-api/internal/services/cover_art_service.go:1586
  STILL go:S1192 CRITICAL catalog-api/internal/services/cover_art_service.go:1672
  STILL go:S1192 CRITICAL catalog-api/internal/services/cover_art_service.go:1672
  STILL go:S1192 CRITICAL catalog-api/internal/services/cover_art_service.go:1676
  STILL go:S1192 CRITICAL catalog-api/internal/services/cover_art_service.go:1678
  STILL go:S1192 CRITICAL catalog-api/internal/services/cover_art_service.go:1701
  STILL go:S1192 CRITICAL catalog-api/main.go:1173
  STILL go:S3776 CRITICAL catalog-api/tests/security/injection_test.go:582
  STILL go:S3776 CRITICAL catalog-api/tests/stress/database_stress_test.go:420
  STILL go:S3776 CRITICAL catalog-api/tests/stress/database_stress_test.go:581
  STILL go:S3776 CRITICAL catalog-api/tests/stress/database_stress_test.go:731
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:684
  STILL go:S3776 CRITICAL catalog-api/internal/services/aggregation_service.go:921
  STILL go:S1192 CRITICAL catalog-api/handlers/media_entity_handler.go:77
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:1278
  STILL go:S3776 CRITICAL catalog-api/repository/media_file_repository.go:184
  STILL go:S1192 CRITICAL catalog-api/main.go:1385
  STILL go:S3776 CRITICAL catalog-api/internal/metrics/snapshot.go:50
  STILL go:S3776 CRITICAL catalog-api/services/reporting_service.go:1108
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:1569
  STILL go:S3776 CRITICAL catalog-api/middleware/request.go:36
  STILL go:S3776 CRITICAL catalog-api/services/configuration_wizard_service.go:1018
  STILL go:S1192 CRITICAL catalog-api/main.go:1503
  STILL go:S1192 CRITICAL catalog-api/challenges/ch251_401.go:82
  STILL go:S1192 CRITICAL catalog-api/challenges/ch251_401.go:88
  STILL go:S3776 CRITICAL catalog-api/challenges/challenge_execute_test.go:21
  STILL go:S3776 CRITICAL catalog-api/handlers/media_browse_handler.go:101
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/stream_handler.go:195
  STILL go:S1192 CRITICAL catalog-api/handlers/cover_handler.go:69
  STILL go:S1192 CRITICAL catalog-api/handlers/cover_handler.go:70
  STILL go:S3776 CRITICAL catalog-api/handlers/media_entity_handler.go:1495
  STILL go:S1192 CRITICAL catalog-api/challenges/ch141_phase1_endpoints.go:36
  STILL go:S3776 CRITICAL catalog-api/challenges/ch141_phase1_endpoints.go:43
  STILL go:S3776 CRITICAL catalog-api/challenges/ch141_phase1_endpoints.go:152
  STILL go:S1192 CRITICAL catalog-api/challenges/ch141_phase1_endpoints.go:680
  STILL go:S1192 CRITICAL catalog-api/challenges/ch153_module_integration.go:34
  STILL go:S1192 CRITICAL catalog-api/challenges/ch153_module_integration.go:35
  STILL go:S1192 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:35
  STILL go:S1192 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:67
  STILL go:S3776 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:123
  STILL go:S1192 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:225
  STILL go:S1192 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:258
  STILL go:S1192 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:265
  STILL go:S1192 CRITICAL catalog-api/challenges/ch159_170_playlist_challenges.go:684
  STILL go:S1192 CRITICAL catalog-api/challenges/ch171_185_media_analysis_challenges.go:33
  STILL go:S1192 CRITICAL catalog-api/challenges/ch171_185_media_analysis_challenges.go:34
  STILL go:S1192 CRITICAL catalog-api/challenges/ch171_185_media_analysis_challenges.go:86
  STILL go:S1192 CRITICAL catalog-api/challenges/ch171_185_media_analysis_challenges.go:357
  STILL go:S1192 CRITICAL catalog-api/challenges/ch186_200_security_challenges.go:37
  STILL go:S1192 CRITICAL catalog-api/challenges/ch186_200_security_challenges.go:307
  STILL go:S1192 CRITICAL catalog-api/challenges/ch201_220_performance_challenges.go:40
  STILL go:S1192 CRITICAL catalog-api/challenges/ch201_220_performance_challenges.go:62
  STILL go:S1192 CRITICAL catalog-api/challenges/ch201_220_performance_challenges.go:337
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:38
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:39
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:72
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:73
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:85
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:370
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:954
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:1088
  STILL go:S1192 CRITICAL catalog-api/challenges/ch221_250_multiplatform_challenges.go:1908
  STILL go:S3776 CRITICAL catalog-api/handlers/stub_handler.go:37
  STILL go:S1192 CRITICAL catalog-api/handlers/stub_handler.go:81
  STILL go:S1192 CRITICAL catalog-api/handlers/stub_handler.go:111
  STILL go:S3776 CRITICAL catalog-api/handlers/stub_handler.go:123
  STILL go:S3776 CRITICAL catalog-api/handlers/stub_handler.go:215
  STILL go:S1192 CRITICAL catalog-api/handlers/stub_handler.go:332
  STILL go:S3776 CRITICAL catalog-api/handlers/stub_handler.go:418
  STILL go:S1192 CRITICAL catalog-api/handlers/playlist_handler.go:41
  STILL go:S1192 CRITICAL catalog-api/handlers/playlist_handler.go:88
  STILL go:S1192 CRITICAL catalog-api/handlers/playlist_handler.go:126
  STILL go:S1192 CRITICAL catalog-api/challenges/ch100_admin_challenges.go:30
  STILL go:S1192 CRITICAL catalog-api/challenges/ch100_admin_challenges.go:61
  STILL go:S3776 CRITICAL catalog-api/challenges/ch100_admin_challenges.go:858
  STILL go:S1192 CRITICAL catalog-api/challenges/ch111_conversion_challenges.go:30
  STILL go:S1192 CRITICAL catalog-api/challenges/ch111_conversion_challenges.go:61
  STILL go:S1192 CRITICAL catalog-api/challenges/ch121_entity_challenges.go:29
  STILL go:S1192 CRITICAL catalog-api/challenges/ch131_middleware_challenges.go:31
  STILL go:S1192 CRITICAL catalog-api/challenges/ch131_middleware_challenges.go:51
  STILL go:S1192 CRITICAL catalog-api/handlers/admin_handler.go:275
  STILL go:S1192 CRITICAL catalog-api/handlers/conversion_handler.go:54
  STILL go:S1192 CRITICAL catalog-api/handlers/conversion_handler.go:93
  STILL go:S3776 CRITICAL catalog-api/tests/stress/cache_stress_test.go:22
  STILL go:S3776 CRITICAL catalog-api/tests/stress/cache_stress_test.go:193
  STILL go:S3776 CRITICAL catalog-api/tests/stress/concurrent_handlers_test.go:93
  STILL go:S3776 CRITICAL catalog-api/tests/stress/repository_stress_test.go:110
  STILL go:S3776 CRITICAL catalog-api/tests/stress/repository_stress_test.go:292
  STILL go:S3776 CRITICAL catalog-api/tests/stress/repository_stress_test.go:530
  STILL go:S3776 CRITICAL catalog-api/services/log_management_service.go:588
  STILL go:S1192 CRITICAL catalog-api/challenges/ch094_098_provider_verification.go:32
  STILL go:S1192 CRITICAL catalog-api/challenges/ch094_098_provider_verification.go:58
  STILL go:S1192 CRITICAL catalog-api/challenges/ch094_098_provider_verification.go:59
  STILL go:S1192 CRITICAL catalog-api/challenges/ch094_098_provider_verification.go:63
  STILL go:S1192 CRITICAL catalog-api/challenges/performance_verification.go:34
  STILL go:S3776 CRITICAL catalog-api/challenges/performance_verification.go:177
  STILL go:S1192 CRITICAL catalog-api/challenges/performance_verification.go:189
  STILL go:S3776 CRITICAL catalog-api/internal/media/providers/providers.go:1034
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/download.go:136
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/download.go:137
  STILL go:S3776 CRITICAL catalog-api/main.go:121
  STILL go:S1192 CRITICAL catalog-api/challenges/ch061_070.go:30
  STILL go:S1192 CRITICAL catalog-api/challenges/ch071_080.go:32
  STILL go:S1192 CRITICAL catalog-api/challenges/ch071_080.go:509
  STILL go:S1192 CRITICAL catalog-api/challenges/ch081_088.go:35
  STILL go:S1192 CRITICAL catalog-api/challenges/ch081_088.go:193
  STILL go:S1192 CRITICAL catalog-api/challenges/ch081_088.go:380
  STILL go:S3776 CRITICAL catalog-api/challenges/ch081_088.go:522
  STILL go:S3776 CRITICAL catalog-api/challenges/ch081_088.go:705
  STILL go:S1192 CRITICAL catalog-api/challenges/module_functional_challenges.go:162
  STILL go:S1192 CRITICAL catalog-api/challenges/module_functional_challenges.go:178
  STILL go:S3776 CRITICAL catalog-api/tests/stress/middleware_chain_stress_test.go:47
  STILL go:S3776 CRITICAL catalog-api/tests/stress/middleware_chain_stress_test.go:156
  STILL go:S3776 CRITICAL catalog-api/challenges/ch051_input_validation.go:39
  STILL go:S3776 CRITICAL catalog-api/challenges/ch053_content_types.go:37
  STILL go:S1192 CRITICAL catalog-api/challenges/ch053_content_types.go:49
  STILL go:S1192 CRITICAL catalog-api/handlers/sync_handler.go:40
  STILL go:S1192 CRITICAL catalog-api/handlers/sync_handler.go:91
  STILL go:S1192 CRITICAL catalog-api/handlers/sync_handler.go:100
  STILL go:S3776 CRITICAL catalog-api/services/configuration_wizard_service_test.go:1416
  STILL go:S3776 CRITICAL catalog-api/internal/services/media_recognition_service.go:997
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:511
  STILL go:S3776 CRITICAL catalog-api/tests/monitoring/resource_monitor_test.go:231
  STILL go:S3776 CRITICAL catalog-api/tests/stress/memory_pressure_test.go:18
  STILL go:S3776 CRITICAL catalog-api/tests/stress/memory_pressure_test.go:122
  STILL go:S3776 CRITICAL catalog-api/tests/stress/memory_pressure_test.go:234
  STILL go:S1192 CRITICAL catalog-api/challenges/ch037_jwt_expiration.go:91
  STILL go:S3776 CRITICAL catalog-api/challenges/ch040_no_sensitive_errors.go:59
  STILL go:S3776 CRITICAL catalog-api/challenges/ch041_health_latency.go:37
  STILL go:S3776 CRITICAL catalog-api/challenges/ch042_file_listing_latency.go:39
  STILL go:S3776 CRITICAL catalog-api/challenges/ch044_websocket_latency.go:40
  STILL go:S3776 CRITICAL catalog-api/challenges/ch045_api_docs.go:37
  STILL go:S3776 CRITICAL catalog-api/challenges/ch046_database_docs.go:39
  STILL go:S3776 CRITICAL catalog-api/challenges/ch047_config_docs.go:39
  STILL go:S3776 CRITICAL catalog-api/challenges/ch050_graceful_shutdown.go:39
  STILL go:S1192 CRITICAL catalog-api/challenges/ch050_graceful_shutdown.go:53
  STILL go:S3776 CRITICAL catalog-api/challenges/coverage_test.go:1752
  STILL go:S3776 CRITICAL catalog-api/database/dialect_fuzz_test.go:8
  STILL go:S3776 CRITICAL catalog-api/database/dialect_fuzz_test.go:79
  STILL go:S3776 CRITICAL catalog-api/cmd/boot/main.go:24
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:50
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:98
  STILL go:S1192 CRITICAL catalog-api/handlers/service_handlers.go:120
  STILL go:S3776 CRITICAL catalog-api/internal/services/media_recognition_service.go:665
  STILL go:S3776 CRITICAL catalog-api/services/configuration_service.go:722
  STILL go:S3776 CRITICAL catalog-api/challenges/browsing_api_catalog.go:39
  STILL go:S3776 CRITICAL catalog-api/challenges/first_catalog_populate.go:40
  STILL go:S3776 CRITICAL catalog-api/challenges/browsing_web_app.go:65
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:135
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:141
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:147
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:158
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:250
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:262
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:268
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:317
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:324
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:390
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:392
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:445
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:655
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:659
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:667
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:740
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:742
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:748
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:957
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:959
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:962
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:995
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1025
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1263
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1265
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1302
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1457
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1459
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1495
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1497
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1675
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1677
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1682
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1853
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1855
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1864
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1972
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:1974
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:2212
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:2214
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:2280
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_web.go:2417
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:95
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:149
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:185
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:189
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:267
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:397
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:630
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:636
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:646
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:814
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:1020
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:1173
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:1178
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_mobile.go:1434
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:95
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:149
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:185
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:233
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:238
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:242
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:248
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:250
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:254
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:256
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:261
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:318
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:323
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:412
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:721
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:790
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:824
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:826
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:835
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:837
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:869
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:871
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:877
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:973
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_desktop.go:996
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:96
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:98
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:149
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:160
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:329
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:638
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:655
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:809
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:820
  STILL go:S1192 CRITICAL catalog-api/challenges/userflow_api.go:1193
  STILL go:S3776 CRITICAL catalog-api/challenges/auth_token_refresh.go:41
  STILL go:S3776 CRITICAL catalog-api/challenges/collection_management.go:37
  STILL go:S1192 CRITICAL catalog-api/challenges/collection_management.go:116
  STILL go:S3776 CRITICAL catalog-api/challenges/config_wizard.go:37
  STILL go:S3776 CRITICAL catalog-api/challenges/cover_art.go:36
  STILL go:S1192 CRITICAL catalog-api/challenges/cover_art.go:118
  STILL go:S3776 CRITICAL catalog-api/challenges/favorites_workflow.go:38
  STILL go:S1192 CRITICAL catalog-api/challenges/favorites_workflow.go:109
  STILL go:S3776 CRITICAL catalog-api/challenges/media_playback.go:36
  STILL go:S1192 CRITICAL catalog-api/challenges/media_playback.go:112
  STILL go:S3776 CRITICAL catalog-api/challenges/search_filter.go:38
  STILL go:S3776 CRITICAL catalog-api/challenges/security_challenge.go:38
  STILL go:S1192 CRITICAL catalog-api/challenges/security_challenge.go:78
  STILL go:S3776 CRITICAL catalog-api/challenges/stress_test_challenge.go:40
  STILL go:S3776 CRITICAL catalog-api/challenges/websocket_events.go:40
  STILL go:S3776 CRITICAL catalog-api/middleware/cache_headers.go:19
  STILL go:S1192 CRITICAL catalog-api/middleware/cache_headers.go:62
  STILL go:S3776 CRITICAL catalog-api/services/reporting_service.go:464
  STILL go:S3776 CRITICAL catalog-api/challenges/collections_api.go:40
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_search.go:39
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_user_metadata.go:40
  STILL go:S3776 CRITICAL catalog-api/challenges/storage_roots_api.go:38
  STILL go:S1192 CRITICAL catalog-api/handlers/media_entity_handler.go:260
  STILL go:S3776 CRITICAL catalog-api/challenges/asset_lazy_loading.go:36
  STILL go:S3776 CRITICAL catalog-api/challenges/asset_serving.go:36
  STILL go:S1192 CRITICAL catalog-api/challenges/asset_serving.go:105
  STILL go:S1192 CRITICAL catalog-api/challenges/browsing_api_catalog.go:132
  STILL go:S1192 CRITICAL catalog-api/challenges/browsing_web_app.go:127
  STILL go:S3776 CRITICAL catalog-api/challenges/database_connectivity.go:48
  STILL go:S3776 CRITICAL catalog-api/challenges/database_schema_validation.go:35
  STILL go:S1192 CRITICAL catalog-api/challenges/database_schema_validation.go:92
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_aggregation.go:38
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_browsing.go:37
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_duplicates.go:37
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_hierarchy.go:37
  STILL go:S3776 CRITICAL catalog-api/challenges/entity_metadata.go:40
  STILL go:S1192 CRITICAL catalog-api/handlers/auth_handler.go:70
  STILL go:S1192 CRITICAL catalog-api/handlers/auth_handler.go:93
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:36
  STILL go:S3776 CRITICAL catalog-api/challenges/browsing_api_health.go:47
  STILL go:S3776 CRITICAL catalog-api/internal/services/recommendation_service.go:626
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:39
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:574
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:630
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:806
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:924
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:1014
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:1246
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:1457
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service_test.go:1540
  STILL go:S3776 CRITICAL catalog-api/tests/performance/protocol_bench_test.go:580
  STILL go:S3776 CRITICAL catalog-api/tests/performance/protocol_bench_test.go:654
  STILL go:S3776 CRITICAL catalog-api/tests/performance/protocol_bench_test.go:694
  STILL go:S3776 CRITICAL catalog-api/tests/stress/database_stress_test.go:274
  STILL go:S3776 CRITICAL catalog-api/internal/services/subtitle_service.go:582
  STILL go:S3776 CRITICAL catalog-api/internal/services/subtitle_service.go:694
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:63
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:246
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:701
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:503
  STILL go:S3776 CRITICAL catalog-api/services/error_reporting_service.go:684
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:699
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:700
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:701
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:701
  STILL go:S3776 CRITICAL catalog-api/tests/performance/baseline_test.go:811
  STILL go:S3776 CRITICAL catalog-api/tests/performance/database_bench_test.go:349
  STILL go:S1192 CRITICAL catalog-api/handlers/media_handler.go:46
  STILL go:S1192 CRITICAL catalog-api/handlers/media_handler.go:87
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:58
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:81
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:206
  STILL go:S3776 CRITICAL catalog-api/internal/services/media_recognition_service.go:380
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:65
  STILL go:S3776 CRITICAL catalog-api/tests/mocks/smb_mock_server.go:247
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/webdav_mock_server.go:70
  STILL go:S3776 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:32
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:42
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:42
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:43
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:43
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:44
  STILL go:S1192 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:44
  STILL go:S3776 CRITICAL catalog-api/internal/tests/video_player_subtitle_logic_test.go:11
  STILL go:S1192 CRITICAL catalog-api/middleware/redis_rate_limiter.go:35
  STILL go:S1192 CRITICAL catalog-api/middleware/redis_rate_limiter.go:172
  STILL go:S1192 CRITICAL catalog-api/middleware/redis_rate_limiter.go:173
  STILL go:S1192 CRITICAL catalog-api/handlers/configuration_handler.go:71
  STILL go:S3776 CRITICAL catalog-api/handlers/log_management_handler_test.go:293
  STILL go:S3776 CRITICAL catalog-api/handlers/role_handler_test.go:71
  STILL go:S3776 CRITICAL catalog-api/handlers/role_handler_test.go:197
  STILL go:S3776 CRITICAL catalog-api/services/favorites_service.go:607
  STILL go:S3776 CRITICAL catalog-api/internal/services/catalog.go:50
  STILL go:S1192 CRITICAL catalog-api/internal/services/catalog.go:105
  STILL go:S1192 CRITICAL catalog-api/handlers/browse.go:72
  STILL go:S3776 CRITICAL catalog-api/internal/media/realtime/enhanced_watcher.go:228
  STILL go:S3776 CRITICAL catalog-api/internal/services/catalog.go:212
  STILL go:S3776 CRITICAL catalog-api/internal/media/analyzer/analyzer.go:532
  STILL go:S1192 CRITICAL catalog-api/internal/services/book_recognition_provider.go:786
  STILL go:S3776 CRITICAL catalog-api/internal/services/recommendation_service.go:916
  STILL go:S3776 CRITICAL catalog-api/services/configuration_wizard_service.go:738
  STILL go:S3776 CRITICAL catalog-api/services/configuration_wizard_service.go:815
  STILL go:S1192 CRITICAL catalog-api/tests/automation/screenshot_automation.go:328
  STILL go:S1192 CRITICAL catalog-api/tests/automation/screenshot_automation.go:536
  STILL go:S1192 CRITICAL catalog-api/handlers/auth_handler.go:306
  STILL go:S1192 CRITICAL catalog-api/handlers/auth_handler.go:312
  STILL go:S1192 CRITICAL catalog-api/handlers/auth_handler.go:325
  STILL go:S1192 CRITICAL catalog-api/handlers/auth_handler.go:325
  STILL go:S1192 CRITICAL catalog-api/handlers/configuration_handler.go:61
  STILL go:S1192 CRITICAL catalog-api/handlers/configuration_handler.go:61
  STILL go:S1192 CRITICAL catalog-api/handlers/configuration_handler.go:167
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:60
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:66
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:89
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:89
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:140
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:503
  STILL go:S3776 CRITICAL catalog-api/handlers/error_reporting_handler.go:517
  STILL go:S1192 CRITICAL catalog-api/handlers/error_reporting_handler.go:533
  STILL go:S3776 CRITICAL catalog-api/handlers/error_reporting_handler.go:559
  STILL go:S3776 CRITICAL catalog-api/handlers/error_reporting_handler.go:597
  STILL go:S1192 CRITICAL catalog-api/handlers/log_management_handler.go:83
  STILL go:S1192 CRITICAL catalog-api/handlers/log_management_handler.go:93
  STILL go:S1192 CRITICAL catalog-api/handlers/log_management_handler.go:93
  STILL go:S1192 CRITICAL catalog-api/handlers/log_management_handler.go:106
  STILL go:S1192 CRITICAL catalog-api/handlers/log_management_handler.go:335
  STILL go:S3776 CRITICAL catalog-api/handlers/log_management_handler.go:555
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:41
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:53
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:58
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:87
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:87
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:118
  STILL go:S1192 CRITICAL catalog-api/handlers/role_handler.go:124
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:186
  STILL go:S1192 CRITICAL catalog-api/handlers/user_handler.go:186
  STILL go:S3776 CRITICAL catalog-api/repository/analytics_repository.go:95
  STILL go:S3776 CRITICAL catalog-api/repository/conversion_repository.go:327
  STILL go:S3776 CRITICAL catalog-api/repository/crash_reporting_repository.go:115
  STILL go:S3776 CRITICAL catalog-api/repository/error_reporting_repository.go:120
  STILL go:S1192 CRITICAL catalog-api/repository/favorites_repository.go:159
  STILL go:S1192 CRITICAL catalog-api/repository/favorites_repository.go:163
  STILL go:S3776 CRITICAL catalog-api/repository/log_management_repository.go:218
  STILL go:S3776 CRITICAL catalog-api/repository/sync_repository.go:263
  STILL go:S1192 CRITICAL catalog-api/services/analytics_service.go:169
  STILL go:S3776 CRITICAL catalog-api/services/configuration_service.go:401
  STILL go:S1192 CRITICAL catalog-api/services/conversion_service.go:749
  STILL go:S1192 CRITICAL catalog-api/services/error_reporting_service.go:378
  STILL go:S3776 CRITICAL catalog-api/services/log_management_service.go:497
  STILL go:S3776 CRITICAL catalog-api/internal/services/duplicate_detection_service.go:939
  STILL go:S3776 CRITICAL catalog-api/internal/tests/recommendation_integration_test.go:249
  STILL go:S3776 CRITICAL catalog-api/internal/services/deep_linking_service.go:163
  STILL go:S1192 CRITICAL catalog-api/internal/services/deep_linking_service.go:166
  STILL go:S1192 CRITICAL catalog-api/internal/services/deep_linking_service.go:243
  STILL go:S1192 CRITICAL catalog-api/internal/services/deep_linking_service.go:354
  STILL go:S3776 CRITICAL catalog-api/internal/services/recommendation_service.go:972
  STILL go:S3776 CRITICAL catalog-api/internal/tests/deep_linking_service_test.go:17
  STILL go:S3776 CRITICAL catalog-api/internal/tests/recommendation_service_test_fixed.go:282
  STILL go:S3776 CRITICAL catalog-api/internal/services/book_recognition_provider.go:1414
  STILL go:S3776 CRITICAL catalog-api/internal/services/duplicate_detection_service.go:695
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:90
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:107
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:121
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:121
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:285
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:325
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:780
  STILL go:S1192 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:781
  STILL go:S3776 CRITICAL catalog-api/internal/tests/media_recognition_mock_servers.go:1256
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/localization_handlers.go:86
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/localization_handlers.go:93
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/localization_handlers.go:284
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/media_player_handlers.go:156
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/media_player_handlers.go:337
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/media_player_handlers.go:598
  STILL go:S1192 CRITICAL catalog-api/internal/services/localization_service.go:499
  STILL go:S1192 CRITICAL catalog-api/internal/services/localization_service.go:705
  STILL go:S1192 CRITICAL catalog-api/internal/services/localization_service.go:707
  STILL go:S3776 CRITICAL catalog-api/internal/services/localization_service.go:836
  STILL go:S3776 CRITICAL catalog-api/internal/services/localization_service.go:1080
  STILL go:S3776 CRITICAL catalog-api/internal/services/localization_service.go:1245
  STILL go:S3776 CRITICAL catalog-api/internal/services/video_player_service.go:784
  STILL go:S3776 CRITICAL catalog-api/internal/services/video_player_service.go:988
  STILL go:S1192 CRITICAL catalog-api/internal/tests/mock_servers.go:98
  STILL go:S1192 CRITICAL catalog-api/internal/tests/mock_servers.go:310
  STILL go:S1192 CRITICAL catalog-api/internal/tests/mock_servers.go:313
  STILL go:S1192 CRITICAL catalog-api/internal/tests/mock_servers.go:530
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/smb_discovery.go:86
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:63
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:67
  STILL go:S3776 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:256
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:306
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:486
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/ftp_mock_server.go:531
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/nfs_mock_server.go:66
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/nfs_mock_server.go:67
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/nfs_mock_server.go:68
  STILL go:S3776 CRITICAL catalog-api/tests/mocks/nfs_mock_server.go:268
  STILL go:S3776 CRITICAL catalog-api/tests/mocks/smb_mock_server.go:393
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/webdav_mock_server.go:79
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/webdav_mock_server.go:89
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/webdav_mock_server.go:274
  STILL go:S1192 CRITICAL catalog-api/tests/mocks/webdav_mock_server.go:285
  STILL go:S3776 CRITICAL catalog-api/internal/services/universal_rename_tracker.go:220
  STILL go:S3776 CRITICAL catalog-api/repository/file_repository.go:433
  STILL go:S3776 CRITICAL catalog-api/tests/integration/protocol_rename_test.go:305
  STILL go:S3776 CRITICAL catalog-api/tests/integration/protocol_rename_test.go:590
  STILL go:S1192 CRITICAL catalog-api/repository/stats_repository.go:241
  STILL go:S1192 CRITICAL catalog-api/internal/auth/middleware.go:62
  STILL go:S1192 CRITICAL catalog-api/internal/auth/middleware.go:69
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/auth.go:40
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/auth.go:143
  STILL go:S1192 CRITICAL catalog-api/internal/handlers/download.go:165
  STILL go:S3776 CRITICAL catalog-api/internal/handlers/media.go:106
  STILL go:S3776 CRITICAL catalog-api/internal/media/detector/engine.go:186
  STILL go:S3776 CRITICAL catalog-api/internal/media/providers/providers.go:455
  STILL go:S3776 CRITICAL catalog-api/internal/recovery/retry.go:71
  STILL go:S1192 CRITICAL catalog-api/repository/stats_repository.go:134
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:324
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:327
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:329
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:333
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:339
  GONE  go:S1186 CRITICAL catalog-api/filesystem/s3_client.go:343
  GONE  go:S1186 CRITICAL catalog-api/internal/services/cover_art_service.go:617
  GONE  go:S1186 CRITICAL catalog-api/services/pdf_page_render.go:73
  GONE  go:S1186 CRITICAL catalog-api/services/conversion_proc_pdeathsig_other.go:9
  GONE  go:S1186 CRITICAL catalog-api/database/encryption.go:524

#### api S3923 bugs: total 2 GONE 2 STILL 0
  GONE  go:S3923 MAJOR catalog-api/services/webdav_client_test.go:594
  GONE  go:S3923 MAJOR catalog-api/services/configuration_wizard_service.go:1179

### web: by rule (run3 -> run4 open)
- typescript:S6853 MAJOR CODE_SMELL: 65 -> 65
- typescript:S5976 MAJOR CODE_SMELL: 47 -> 47
- typescript:S7773 MINOR CODE_SMELL: 34 -> 34
- typescript:S5906 MINOR CODE_SMELL: 34 -> 34
- typescript:S3358 MAJOR CODE_SMELL: 33 -> 32
- typescript:S6551 MINOR CODE_SMELL: 21 -> 21
- typescript:S2699 BLOCKER CODE_SMELL: 16 -> 0
- typescript:S6479 MAJOR CODE_SMELL: 16 -> 16
- typescript:S6478 MAJOR CODE_SMELL: 9 -> 9
- typescript:S6754 MINOR CODE_SMELL: 9 -> 9
- typescript:S1082 MINOR BUG: 9 -> 0
- typescript:S6759 MINOR CODE_SMELL: 9 -> 9
- typescript:S3776 CRITICAL CODE_SMELL: 8 -> 0
- typescript:S6848 MAJOR CODE_SMELL: 8 -> 0
- typescript:S7762 MAJOR CODE_SMELL: 7 -> 7
- typescript:S6582 MINOR CODE_SMELL: 7 -> 7
- typescript:S1874 MINOR CODE_SMELL: 5 -> 1
- typescript:S1128 MINOR CODE_SMELL: 5 -> 5
- typescript:S2245 MAJOR VULNERABILITY: 5 -> 0
- typescript:S1854 MAJOR CODE_SMELL: 5 -> 5
- typescript:S3735 CRITICAL CODE_SMELL: 4 -> 0
- typescript:S1444 MINOR CODE_SMELL: 4 -> 4
- typescript:S6767 MINOR CODE_SMELL: 3 -> 3
- typescript:S2486 MINOR CODE_SMELL: 3 -> 3
- typescript:S6481 MAJOR CODE_SMELL: 3 -> 3
- typescript:S2933 MAJOR CODE_SMELL: 2 -> 2
- typescript:S5843 MAJOR CODE_SMELL: 2 -> 2
- typescript:S7741 MINOR CODE_SMELL: 2 -> 2
- typescript:S6660 MAJOR CODE_SMELL: 2 -> 2
- typescript:S8985 MAJOR BUG: 2 -> 0
- typescript:S4323 MINOR CODE_SMELL: 2 -> 3
- typescript:S7756 MINOR CODE_SMELL: 2 -> 2
- typescript:S6959 MAJOR BUG: 2 -> 0
- typescript:S6594 MINOR CODE_SMELL: 1 -> 1
- typescript:S6850 MAJOR CODE_SMELL: 1 -> 1
- typescript:S6847 MAJOR CODE_SMELL: 1 -> 0
- typescript:S7750 MINOR CODE_SMELL: 1 -> 1
- typescript:S7745 MINOR CODE_SMELL: 1 -> 1
- typescript:S6564 MAJOR CODE_SMELL: 1 -> 1
- typescript:S3923 MAJOR BUG: 1 -> 0
- typescript:S1940 MINOR CODE_SMELL: 1 -> 1
- typescript:S8786 MAJOR CODE_SMELL: 1 -> 1
- typescript:S8754 MAJOR CODE_SMELL: 1 -> 1
- typescript:S1313 MINOR CODE_SMELL: 1 -> 1
- typescript:S9115 MAJOR BUG: 1 -> 0
- typescript:S4084 MINOR CODE_SMELL: 1 -> 1
- typescript:S7761 MAJOR CODE_SMELL: 1 -> 1
- typescript:S4043 MAJOR CODE_SMELL: 1 -> 1
- typescript:S7758 MINOR CODE_SMELL: 1 -> 1
- typescript:S7770 MINOR CODE_SMELL: 1 -> 1
- typescript:S6819 MAJOR CODE_SMELL: 1 -> 9
- typescript:S7721 MAJOR CODE_SMELL: 1 -> 1
NEW (13): typescript:S6819 MAJOR catalog-web/src/components/collections/CollectionTemplates.tsx:497; typescript:S4323 MINOR catalog-web/src/components/favorites/FavoriteToggle.tsx:11; typescript:S3358 MAJOR catalog-web/src/components/favorites/FavoriteToggle.tsx:134; typescript:S7773 MINOR catalog-web/src/lib/collectionRules.ts:358; typescript:S7773 MINOR catalog-web/src/lib/collectionRules.ts:361; typescript:S7773 MINOR catalog-web/src/lib/collectionRules.ts:384; typescript:S6819 MAJOR catalog-web/src/components/collections/AdvancedSearch.tsx:488; typescript:S6819 MAJOR catalog-web/src/components/collections/AdvancedSearch.tsx:532; typescript:S6819 MAJOR catalog-web/src/components/collections/CollectionExport.tsx:723; typescript:S6819 MAJOR catalog-web/src/components/collections/CollectionPreview.tsx:216; typescript:S6819 MAJOR catalog-web/src/components/identity/DiscoveredShares.tsx:151; typescript:S6819 MAJOR catalog-web/src/components/media/HistoryDrawer.tsx:66; typescript:S6819 MAJOR catalog-web/src/pages/SubtitleManager.tsx:278

#### web CRITICAL: total 12 GONE 12 STILL 0
  GONE  typescript:S3776 CRITICAL catalog-web/src/test/contrast.ts:147
  GONE  typescript:S3776 CRITICAL catalog-web/src/test/contrast.ts:118
  GONE  typescript:S3776 CRITICAL catalog-web/src/components/collections/CollectionRealTime.tsx:112
  GONE  typescript:S3776 CRITICAL catalog-web/src/test/contrast.ts:55
  GONE  typescript:S3735 CRITICAL catalog-web/src/components/auth/RegisterForm.tsx:93
  GONE  typescript:S3735 CRITICAL catalog-web/src/components/collections/BulkOperations.tsx:137
  GONE  typescript:S3735 CRITICAL catalog-web/src/components/collections/CollectionSettings.tsx:159
  GONE  typescript:S3735 CRITICAL catalog-web/src/components/collections/CollectionSettings.tsx:184
  GONE  typescript:S3776 CRITICAL catalog-web/src/lib/collectionRules.ts:342
  GONE  typescript:S3776 CRITICAL catalog-web/src/pages/Collections.tsx:104
  GONE  typescript:S3776 CRITICAL catalog-web/src/components/favorites/FavoriteToggle.tsx:30
  GONE  typescript:S3776 CRITICAL catalog-web/src/components/favorites/FavoritesGrid.tsx:41

#### web vulns S2245: total 5 GONE 5 STILL 0
  GONE  typescript:S2245 MAJOR catalog-web/src/test-setup.ts:132
  GONE  typescript:S2245 MAJOR catalog-web/src/test-setup.ts:135
  GONE  typescript:S2245 MAJOR catalog-web/src/components/collections/AdvancedSearch.tsx:199
  GONE  typescript:S2245 MAJOR catalog-web/src/components/collections/SmartCollectionBuilder.tsx:100
  GONE  typescript:S2245 MAJOR catalog-web/src/components/upload/UploadManager.tsx:50

#### web bugs: total 15 GONE 15 STILL 0
  GONE  typescript:S1082 MINOR catalog-web/src/components/identity/DiscoveredShares.tsx:150
  GONE  typescript:S3923 MAJOR catalog-web/src/components/identity/IdentityManager.tsx:203
  GONE  typescript:S1082 MINOR catalog-web/src/components/media/HistoryDrawer.tsx:66
  GONE  typescript:S1082 MINOR catalog-web/src/components/media/HistoryDrawer.tsx:71
  GONE  typescript:S6959 MAJOR catalog-web/src/components/performance/MemoCache.tsx:40
  GONE  typescript:S1082 MINOR catalog-web/src/components/collections/AdvancedSearch.tsx:486
  GONE  typescript:S1082 MINOR catalog-web/src/components/collections/AdvancedSearch.tsx:527
  GONE  typescript:S1082 MINOR catalog-web/src/components/collections/CollectionTemplates.tsx:493
  GONE  typescript:S1082 MINOR catalog-web/src/components/collections/CollectionExport.tsx:722
  GONE  typescript:S1082 MINOR catalog-web/src/components/collections/CollectionPreview.tsx:215
  GONE  typescript:S6959 MAJOR catalog-web/src/pages/Favorites.tsx:196
  GONE  typescript:S1082 MINOR catalog-web/src/pages/SubtitleManager.tsx:277
  GONE  typescript:S8985 MAJOR catalog-web/src/pages/__tests__/MediaBrowser.test.tsx:475
  GONE  typescript:S8985 MAJOR catalog-web/src/pages/__tests__/MediaBrowser.test.tsx:375
  GONE  typescript:S9115 MAJOR catalog-web/src/pages/__tests__/MediaBrowser.test.tsx:375

#### web S2699 BLOCKER: total 16 GONE 16 STILL 0
  GONE  typescript:S2699 BLOCKER catalog-web/src/__tests__/useCoverQuality.test.tsx:97
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/media/__tests__/HistoryDrawer.test.tsx:210
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/collections/__tests__/CollectionExport.test.tsx:147
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/collections/__tests__/SmartCollectionBuilder.test.tsx:138
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/collections/__tests__/SmartCollectionBuilder.test.tsx:233
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/playlists/__tests__/PlaylistPlayer.test.tsx:147
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/playlists/__tests__/PlaylistPlayer.test.tsx:161
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/playlists/__tests__/PlaylistPlayer.test.tsx:177
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/playlists/__tests__/PlaylistPlayer.test.tsx:196
  GONE  typescript:S2699 BLOCKER catalog-web/src/pages/__tests__/Settings.test.tsx:253
  GONE  typescript:S2699 BLOCKER catalog-web/src/pages/__tests__/Settings.test.tsx:309
  GONE  typescript:S2699 BLOCKER catalog-web/src/pages/__tests__/SubtitleManager.test.tsx:154
  GONE  typescript:S2699 BLOCKER catalog-web/src/pages/__tests__/SubtitleManager.test.tsx:162
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/collections/__tests__/CollectionExport.test.tsx:109
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/playlists/__tests__/PlaylistItem.test.tsx:70
  GONE  typescript:S2699 BLOCKER catalog-web/src/components/playlists/__tests__/PlaylistPlayer.test.tsx:111
