"""compose_mutants.py - the mutants of tests/infra/compose_scan.py (WF17 TI-G1). Each is ONE textual edit of a real compose file (every anchor must occur exactly once, else the mutant is BROKEN and the
battery fails). expect='caught' (default) = the scanner MUST report at least one finding; expect='survive' = an IDENTITY mutant: the scanner MUST report none (a scanner that fails on an equivalent
file is the 11.4.201(1) false-positive refusal). Provenance of the shapes: WF17 reviewer WF17-A1..A7, inputs lens S1..S21, proof lens CM0 (control), CM1..CM8, CM10, and the earlier WF12 mutants (RM1..RM7 ...).
`hunter2literal` is a fixture, never a real credential."""
INFRA = "docker-compose.test-infra.yml"
NFS = "docker-compose.test-infra.nfs.yml"
BUILD = "docker-compose.build.yml"
REDIS_IMG = "docker.io/library/redis@sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499"
SMB_IMG = "docker.io/dperson/samba@sha256:66088b78a19810dd1457a8f39340e95e663c728083efa5fe7dc0d40b2478e869"
REDIS_CMD = '''    command: ["sh", "-c", "exec redis-server --save '' --appendonly no --requirepass \\"$$REDIS_PASSWORD\\""]'''
REDIS_HEAD = "  redis:\n    image: %s\n    pull_policy: never\n    labels: *labels\n" % REDIS_IMG
SMB_HEAD = "  smb:\n    image: %s\n    pull_policy: never\n    labels: *labels\n" % SMB_IMG
PG_ENV_TAIL = '''      POSTGRES_USER: "${TI_POSTGRES_USER:?}"
      POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"
    ports:'''
M = []


def m(name, file, *edits, expect="caught", scan=None):
    d = dict(name=name, file=file, edits=[tuple(edits[i:i + 2]) for i in range(0, len(edits), 2)], expect=expect)
    if scan:
        d["scan"] = scan
    M.append(d)


# ---- WF17 reviewer A1-A7 ----
m("A1_literal_after_ref", INFRA, 'USER: "${TI_SMB_USER:?};${TI_SMB_PASSWORD:?}"', 'USER: "${TI_SMB_USER:?};hunter2literal"')
m("A2_default_literal", INFRA, '      PASSWORD: "${TI_WEBDAV_PASSWORD:?}"', '      PASSWORD: "${TI_WEBDAV_PASSWORD:-hunter2literal}"')
m("A3_dsn_with_password", INFRA, PG_ENV_TAIL, PG_ENV_TAIL.replace("    ports:", '      APP_DSN: "host=postgres user=app password=hunter2literal"\n    ports:'))
m("A4_redis_no_requirepass_infra", INFRA, REDIS_CMD, '''    command: ["sh", "-c", "exec redis-server --save '' --appendonly no"]''')
m("A4_redis_no_requirepass_build", BUILD, REDIS_CMD, '''    command: ["sh", "-c", "exec redis-server --save '' --appendonly no"]''')
m("A5_healthcheck_curl_credential", INFRA, '      - "127.0.0.1:${TI_PORT_WEBDAV:?}:80"\n', '      - "127.0.0.1:${TI_PORT_WEBDAV:?}:80"\n    healthcheck:\n      test: ["CMD-SHELL", "curl -fsu admin:hunter2literal http://localhost/"]\n')
m("A6_port_default", INFRA, '"127.0.0.1:${TI_PORT_REDIS:?}:6379"', '"127.0.0.1:${TI_PORT_REDIS:-6379}:6379"')
m("A7_extra_service", INFRA, "  minio:\n    image:", "  extra:\n    image: %s\n    pull_policy: never\n    privileged: true\n    network_mode: host\n    cap_add: [SYS_ADMIN]\n    ports:\n      - \"0.0.0.0:6380:6379\"\n    labels: *labels\n\n  minio:\n    image:" % REDIS_IMG)
# ---- inputs lens S1-S21 ----
m("S1_exec_form_requirepass", INFRA, REDIS_CMD, '    command: ["redis-server", "--requirepass", "hunter2literal"]')
m("S2_healthcheck_exec_dash_a", INFRA, 'test: ["CMD-SHELL", "redis-cli ping | grep -qx PONG"]', 'test: ["CMD", "redis-cli", "-a", "hunter2literal", "ping"]')
m("S3_url_userinfo_env", INFRA, '      REDISCLI_AUTH: "${TI_REDIS_PASSWORD:?}"\n    command:', '      REDISCLI_AUTH: "${TI_REDIS_PASSWORD:?}"\n      APP_URL: "redis://:hunter2literal@redis"\n    command:')
m("S4_url_password_with_slash", INFRA, '      REDISCLI_AUTH: "${TI_REDIS_PASSWORD:?}"\n    command:', '      REDISCLI_AUTH: "${TI_REDIS_PASSWORD:?}"\n      APP_URL: "redis://:hunter2/lit@redis"\n    command:')
m("S5_account_key", INFRA, '      PERMISSIONS: "yes"\n', '      PERMISSIONS: "yes"\n      ACCOUNT_admin: hunter2literal\n')
m("S6_smb_passwd_key", INFRA, '      PERMISSIONS: "yes"\n', '      PERMISSIONS: "yes"\n      SMB_PASSWD: hunter2literal\n')
m("S7_ftp_pass_in_command", INFRA, "    cap_add:\n      - AUDIT_WRITE\n", '    cap_add:\n      - AUDIT_WRITE\n    command: ["sh", "-c", "FTP_PASS=hunter2literal exec pure-ftpd"]\n')
m("S8_env_file_indirection", INFRA, REDIS_HEAD, REDIS_HEAD + "    env_file: [secrets.env]\n")
m("S9_build_args_credential", BUILD, "      dockerfile: docker/Dockerfile.builder\n", "      dockerfile: docker/Dockerfile.builder\n      args:\n        TOKEN: hunter2literal\n")
m("S10_label_key_password", INFRA, "x-labels: &labels\n  project: catalogizer\n", "x-labels: &labels\n  password: hunter2literal\n  project: catalogizer\n")
m("S11_non_ti_var_default", INFRA, PG_ENV_TAIL, PG_ENV_TAIL.replace("    ports:", '      PGPASSWORD: "${PGPASSWORD:-}"\n    ports:'))
m("S12_integer_in_exec_form", INFRA, 'test: ["CMD-SHELL", "redis-cli ping | grep -qx PONG"]', 'test: ["CMD", "redis-cli", "-a", 12345678, "ping"]')
m("S13_pid_host", INFRA, REDIS_HEAD, REDIS_HEAD + "    pid: host\n")
m("S14_devices", INFRA, REDIS_HEAD, REDIS_HEAD + "    devices:\n      - /dev/fuse\n")
m("S15_security_opt", INFRA, REDIS_HEAD, REDIS_HEAD + "    security_opt:\n      - seccomp=unconfined\n      - label=disable\n")
m("S16_bind_root", INFRA, REDIS_HEAD, REDIS_HEAD + '    volumes:\n      - "/:/host:ro"\n')
m("S17_userns_host", INFRA, REDIS_HEAD, REDIS_HEAD + "    userns_mode: host\n")
m("S18_ipc_host", INFRA, REDIS_HEAD, REDIS_HEAD + "    ipc: host\n")
m("S19_requirepass_literal_dollar", INFRA, REDIS_CMD, '''    command: ["sh", "-c", "exec redis-server --save '' --appendonly no --requirepass '$$lit'"]''')
m("S20_podman_socket_bind", INFRA, '      - "${TI_DATA_DIR:?}/smb:/test-data"', '      - "${TI_DATA_DIR:?}/smb:/test-data"\n      - "/run/podman/podman.sock:/var/run/docker.sock"')
# ---- proof lens CM0 (control: literal), CM1-CM8, CM10 ----
m("CM0_control_literal_postgres_password", INFRA, '      POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"', '      POSTGRES_PASSWORD: "hunter2literal"')
m("CM1_rediscli_auth_literal", INFRA, '      REDISCLI_AUTH: "${TI_REDIS_PASSWORD:?}"', '      REDISCLI_AUTH: "hunter2literal"')
m("CM2_pgpassword_literal", INFRA, PG_ENV_TAIL, PG_ENV_TAIL.replace("    ports:", "      PGPASSWORD: hunter2literal\n    ports:"))
m("CM3_healthcheck_redis_url", INFRA, 'test: ["CMD-SHELL", "redis-cli ping | grep -qx PONG"]', 'test: ["CMD-SHELL", "redis-cli -u redis://:hunter2literal@localhost:6379 ping | grep -qx PONG"]')
m("CM4_healthcheck_pgpassword", INFRA, 'psql -U \\"$$POSTGRES_USER\\"', "PGPASSWORD='hunter2literal' psql -U \\\"$$POSTGRES_USER\\\"")
m("CM5_owned_service_privilege_set", INFRA, SMB_HEAD, SMB_HEAD + "    devices:\n      - /dev/fuse\n    security_opt:\n      - seccomp=unconfined\n      - label=disable\n    pid: host\n    userns_mode: host\n")
m("CM6_labels_merge_password", INFRA, REDIS_HEAD, REDIS_HEAD.replace("labels: *labels", "labels: {<<: *labels, password: hunter2literal}"))
m("CM7_smb_volume_root_and_socket", INFRA, '      - "${TI_DATA_DIR:?}/smb:/test-data"', '      - "${TI_DATA_DIR:?}/smb:/test-data"\n      - "/:/host:ro"\n      - "/var/run/docker.sock:/var/run/docker.sock"')
m("CM8_pull_always", INFRA, "  redis:\n    image: %s\n    pull_policy: never" % REDIS_IMG, "  redis:\n    image: %s\n    pull_policy: always" % REDIS_IMG)
m("CM10_hardcoded_project_label", INFRA, '  catalogizer.test_project: "${TI_PROJECT:?TI_PROJECT is generated by scripts/test-infra/gen_env.sh}"', '  catalogizer.test_project: "catalogizer-test-fixed"')
# ---- the WF12 mutants (RM1..RM7) and the earlier T129 set ----
m("RM1_literal_in_command", INFRA, '\\"$$REDIS_PASSWORD\\"', "hunter2literal")
m("RM2_dsn_in_neutral_env", INFRA, '      PASSWORD: "${TI_WEBDAV_PASSWORD:?}"', '      PASSWORD: "${TI_WEBDAV_PASSWORD:?}"\n      DAV_DSN: "http://admin:hunter2literal@webdav/"')
m("RM3_literal_in_healthcheck", INFRA, "redis-cli ping | grep -qx PONG", "redis-cli -a hunter2literal ping | grep -qx PONG")
m("RM5_trust_auth", INFRA, PG_ENV_TAIL, PG_ENV_TAIL.replace("    ports:", "      POSTGRES_HOST_AUTH_METHOD: trust\n    ports:"))
m("RM7_audit_write_removed", INFRA, "    cap_add:\n      - AUDIT_WRITE\n", "")
m("tag_not_digest", INFRA, REDIS_IMG, "docker.io/library/redis:7-alpine")
m("digest_not_in_lock", INFRA, REDIS_IMG, "docker.io/library/redis@sha256:" + "0" * 64)
m("literal_host_port", INFRA, '"127.0.0.1:${TI_PORT_REDIS:?}:6379"', '"6379:6379"')
m("port_not_loopback", INFRA, '"127.0.0.1:${TI_PORT_REDIS:?}:6379"', '"0.0.0.0:${TI_PORT_REDIS:?}:6379"')
m("privileged_service", INFRA, "    cap_add:\n      - AUDIT_WRITE\n", "    privileged: true\n    cap_add:\n      - AUDIT_WRITE\n")
m("label_project_removed", INFRA, "x-labels: &labels\n  project: catalogizer\n", "x-labels: &labels\n")
m("label_root_removed", INFRA, '  catalogizer.test_root: "${TI_ROOT_HASH:?TI_ROOT_HASH is generated by scripts/test-infra/gen_env.sh}"\n', "")
m("kernel_nfs_back", INFRA, "  minio:\n    image:", "  nfs:\n    image: docker.io/erichough/nfs-server@sha256:%s\n    labels: *labels\n\n  minio:\n    image:" % ("0" * 64))
m("in_pod_enabled_infra", INFRA, "x-podman:\n  in_pod: false\n", "")
m("in_pod_enabled_build", BUILD, "x-podman:\n  in_pod: false\n", "")
m("network_label_root_removed", INFRA, '      catalogizer.test_root: "${TI_ROOT_HASH:?}"\n', "")
m("minio_literal_credential", INFRA, '      MINIO_ROOT_PASSWORD: "${TI_MINIO_ROOT_PASSWORD:?}"', "      MINIO_ROOT_PASSWORD: hunter2literal")
# ---- the nfs file (scanned by nothing before WF17) ----
NFS_HEAD = "    pull_policy: never\n    labels: *labels\n"
m("NFS_devices", NFS, NFS_HEAD, NFS_HEAD + "    devices:\n      - /dev/fuse\n")
m("NFS_privileged_and_cap", NFS, NFS_HEAD, NFS_HEAD + "    privileged: true\n    cap_add: [SYS_ADMIN]\n")
m("NFS_literal_env", NFS, NFS_HEAD, NFS_HEAD + "    environment:\n      EXPORT_PASSWORD: hunter2literal\n")
m("NFS_published_port", NFS, NFS_HEAD, NFS_HEAD + '    ports:\n      - "0.0.0.0:2049:2049"\n')
m("NFS_label_missing_root", NFS, '  catalogizer.test_root: "${TI_ROOT_HASH:?TI_ROOT_HASH is generated by scripts/test-infra/gen_env.sh}"\n', "")
m("NFS_in_pod_enabled", NFS, "x-podman:\n  in_pod: false\n", "", scan=["nfs"])
m("NFS_image_tag_default", NFS, '"${TI_NFS_IMAGE:-localhost/blocked-nfs-image-not-built:none}"', '"${TI_NFS_IMAGE:-localhost/nfs:latest}"')
m("NFS_pull_always", NFS, NFS_HEAD, "    pull_policy: always\n    labels: *labels\n")
# ---- the build compose ----
m("BUILD_redis_published_without_loopback", BUILD, '      - "127.0.0.1:${TI_PORT_REDIS:?}:6379"', '      - "${TI_PORT_REDIS:?}:6379"')
m("BUILD_builder_literal_credential", BUILD, '      DB_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"', "      DB_PASSWORD: hunter2literal")
m("BUILD_op_label_missing", BUILD, '  catalogizer.op_id: "${TI_OP_ID:?TI_OP_ID is generated by scripts/test-infra/gen_env.sh}"\n', "")
# ---- identity mutants: an equivalent file MUST NOT be flagged (11.4.201(1)) ----
m("ID_comment_only", INFRA, "\nnetworks:\n  test-network:\n    name:", "\n# identity mutant: a comment only\nnetworks:\n  test-network:\n    name:", expect="survive")
m("ID_cpu_limit_is_free", INFRA, "    cpus: 0.5\n", "    cpus: 0.75\n", expect="survive")
m("ID_build_comment_only", BUILD, "services:\n", "# identity mutant\nservices:\n", expect="survive")
m("ID_nfs_comment_only", NFS, "services:\n", "# identity mutant\nservices:\n", expect="survive", scan=["nfs"])
MUTANTS = M
