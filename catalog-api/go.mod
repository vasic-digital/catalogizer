module catalogizer

go 1.25.7

replace digital.vasic.challenges => ../submodules/challenges

replace digital.vasic.assets => ../submodules/assets

replace digital.vasic.containers => ../submodules/containers

replace digital.vasic.concurrency => ../submodules/concurrency

replace digital.vasic.config => ../submodules/config

replace digital.vasic.filesystem => ../submodules/filesystem

replace digital.vasic.auth => ../submodules/auth

replace digital.vasic.cache => ../submodules/cache

replace digital.vasic.entities => ../submodules/entities

replace digital.vasic.eventbus => ../submodules/event_bus

replace digital.vasic.database => ../submodules/database

replace digital.vasic.discovery => ../submodules/discovery

replace digital.vasic.lazy => ../submodules/lazy

replace digital.vasic.media => ../submodules/media

replace digital.vasic.memory => ../submodules/memory

replace digital.vasic.middleware => ../submodules/middleware

replace digital.vasic.observability => ../submodules/observability

replace digital.vasic.ratelimiter => ../submodules/rate_limiter

replace digital.vasic.recovery => ../submodules/recovery

replace digital.vasic.security => ../submodules/security

replace digital.vasic.storage => ../submodules/storage

replace digital.vasic.streaming => ../submodules/streaming

replace digital.vasic.watcher => ../submodules/watcher

require (
	cloud.google.com/go/storage v1.62.1
	digital.vasic.assets v0.0.0-00010101000000-000000000000
	digital.vasic.auth v0.0.0-00010101000000-000000000000
	digital.vasic.cache v0.0.0-00010101000000-000000000000
	digital.vasic.challenges v0.0.0-00010101000000-000000000000
	digital.vasic.concurrency v0.0.0
	digital.vasic.config v0.0.0-00010101000000-000000000000
	digital.vasic.containers v0.0.0-00010101000000-000000000000
	digital.vasic.database v0.0.0-00010101000000-000000000000
	digital.vasic.discovery v0.0.0-00010101000000-000000000000
	digital.vasic.entities v0.0.0-00010101000000-000000000000
	digital.vasic.eventbus v0.0.0-00010101000000-000000000000
	digital.vasic.filesystem v0.0.0-00010101000000-000000000000
	digital.vasic.lazy v0.0.0-00010101000000-000000000000
	digital.vasic.media v0.0.0-00010101000000-000000000000
	digital.vasic.memory v0.0.0-00010101000000-000000000000
	digital.vasic.middleware v0.0.0-00010101000000-000000000000
	digital.vasic.observability v0.0.0-00010101000000-000000000000
	digital.vasic.ratelimiter v0.0.0-00010101000000-000000000000
	digital.vasic.recovery v0.0.0-00010101000000-000000000000
	digital.vasic.security v0.0.0-00010101000000-000000000000
	digital.vasic.storage v0.0.0-00010101000000-000000000000
	digital.vasic.streaming v0.0.0-00010101000000-000000000000
	digital.vasic.watcher v0.0.0-00010101000000-000000000000
	firebase.google.com/go/v4 v4.20.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/alicebob/miniredis/v2 v2.36.1
	github.com/andybalholm/brotli v1.2.1
	github.com/aws/aws-sdk-go-v2 v1.41.1
	github.com/aws/aws-sdk-go-v2/config v1.32.7
	github.com/aws/aws-sdk-go-v2/service/s3 v1.96.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gen2brain/go-fitz v1.24.15
	github.com/gin-gonic/gin v1.12.0
	github.com/go-playground/validator/v10 v10.30.1
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/hirochachacha/go-smb2 v1.1.0
	github.com/jlaffaye/ftp v0.2.4
	github.com/lib/pq v1.10.9
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/mutecomm/go-sqlcipher v0.0.0-20190227152316-55dbde17881f
	github.com/nwaples/rardecode/v2 v2.2.5
	github.com/odwrtw/fanarttv v0.0.0-20170412122542-9f67d3cf0188
	github.com/pborman/uuid v1.2.1
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/quic-go/quic-go v0.59.0
	github.com/redis/go-redis/v9 v9.17.3
	github.com/stretchr/testify v1.12.1
	github.com/studio-b12/gowebdav v0.12.0
	github.com/unidoc/unipdf/v3 v3.69.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	golang.org/x/crypto v0.54.0
	golang.org/x/image v0.43.0
	golang.org/x/net v0.56.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.22.0
	golang.org/x/text v0.40.0
	google.golang.org/api v0.279.0
	gopkg.in/mineo/gocaa.v1 v1.0.0-20180225115936-2500f801cd83
)

require (
	cel.dev/expr v0.25.2 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/firestore v1.22.0 // indirect
	cloud.google.com/go/iam v1.11.0 // indirect
	cloud.google.com/go/longrunning v1.0.0 // indirect
	cloud.google.com/go/monitoring v1.29.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.32.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.56.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.56.0 // indirect
	github.com/MicahParks/keyfunc v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.37.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/geoffgarside/ber v1.1.0 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.15 // indirect
	github.com/googleapis/gax-go/v2 v2.22.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gorilla/i18n v0.0.0-20150820051429-8b358169da46 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jupiterrider/ffi v0.5.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/minio-go/v7 v7.0.100 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	github.com/unidoc/freetype v0.2.3 // indirect
	github.com/unidoc/pkcs7 v0.2.0 // indirect
	github.com/unidoc/timestamp v0.0.0-20200412005513-91597fd3793a // indirect
	github.com/unidoc/unichart v0.4.0 // indirect
	github.com/unidoc/unitype v0.5.1 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.43.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.68.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/arch v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/appengine/v2 v2.0.6 // indirect
	google.golang.org/genproto v0.0.0-20260511170946-3700d4141b60 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260511170946-3700d4141b60 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260511170946-3700d4141b60 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
