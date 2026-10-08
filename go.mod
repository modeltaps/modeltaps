module github.com/modeltaps/modeltaps

// +heroku goVersion go1.18
go 1.26.0

// Release, image and CI builds compile with this toolchain; the go line above
// stays the minimum. Keep it on the latest patch: go get toolchain@patch
toolchain go1.27.1

require (
	github.com/ThinkInAIXYZ/go-mcp v0.2.29
	github.com/aliyun/aliyun-oss-go-sdk v3.0.2+incompatible
	github.com/anknown/ahocorasick v0.0.0-20190904063843-d75dbd5169c0
	github.com/aws/aws-sdk-go-v2 v1.47.1
	github.com/aws/aws-sdk-go-v2/credentials v1.20.6
	github.com/aws/aws-sdk-go-v2/service/bedrockruntime v1.63.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.113.4
	github.com/aws/smithy-go v1.28.2
	github.com/bwmarrin/snowflake v0.3.0
	github.com/bytedance/gopkg v0.1.4
	github.com/coocood/freecache v1.2.7
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/eko/gocache/lib/v4 v4.4.0
	github.com/eko/gocache/store/freecache/v4 v4.2.10
	github.com/eko/gocache/store/redis/v4 v4.2.12
	github.com/gin-contrib/cors v1.7.9
	github.com/gin-contrib/gzip v1.2.8
	github.com/gin-contrib/sessions v1.1.2
	github.com/gin-contrib/static v1.1.8
	github.com/gin-gonic/gin v1.12.0
	github.com/go-co-op/gocron/v2 v2.22.0
	github.com/go-gormigrate/gormigrate/v2 v2.1.7
	github.com/go-playground/validator/v10 v10.30.5
	github.com/go-redsync/redsync/v4 v4.18.0
	github.com/go-webauthn/webauthn v0.18.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/gomarkdown/markdown v0.0.0-20260923180740-94fc73f6b1a3
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/mitchellh/mapstructure v1.5.0
	github.com/pkoukk/tiktoken-go v0.1.8
	github.com/prometheus/client_golang v1.24.1
	github.com/redis/go-redis/v9 v9.22.0
	github.com/samber/lo v1.53.0
	github.com/shopspring/decimal v1.4.0
	github.com/smartwalle/alipay/v3 v3.2.31
	github.com/spf13/viper v1.21.0
	github.com/sqids/sqids-go v0.4.1
	github.com/stretchr/testify v1.12.1
	github.com/stripe/stripe-go/v86 v86.4.2
	github.com/tidwall/gjson v1.19.0
	github.com/tidwall/sjson v1.2.5
	github.com/vmihailenco/msgpack/v5 v5.4.1
	github.com/wechatpay-apiv3/wechatpay-go v0.2.21
	github.com/wneessen/go-mail v0.8.1
	go.uber.org/zap v1.28.0
	golang.org/x/crypto v0.57.0
	golang.org/x/image v0.46.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sync v0.23.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gorm.io/driver/mysql v1.6.0
	gorm.io/driver/postgres v1.6.3
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/anknown/darts v0.0.0-20151216065714-83ff685239e6 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic/loader v0.5.2 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.4 // indirect
	github.com/go-jose/go-jose/v4 v4.1.5 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/x v0.3.1 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.63.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/smartwalle/ncrypto v1.0.4 // indirect
	github.com/smartwalle/ngx v1.1.2 // indirect
	github.com/smartwalle/nsign v1.0.9 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.2 // indirect
	github.com/tinylib/msgp v1.6.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.mongodb.org/mongo-driver/v2 v2.9.1 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20260908205506-85c1c2202aba // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

require (
	github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.36
	github.com/bytedance/sonic v1.15.4 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.15 // indirect
	github.com/gin-contrib/sse v1.1.2 // indirect
	github.com/go-playground/locales v0.14.2 // indirect
	github.com/go-playground/universal-translator v0.18.2 // indirect
	github.com/go-sql-driver/mysql v1.10.1 // indirect
	github.com/goccy/go-json v0.11.2 // indirect
	github.com/gorilla/context v1.1.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/sessions v1.4.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.11.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/leodido/go-urn v1.5.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-sqlite3 v1.14.52 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.2 // indirect
	golang.org/x/arch v0.31.0 // indirect
	golang.org/x/net v0.59.0
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0
	gorm.io/datatypes v1.2.7
)
