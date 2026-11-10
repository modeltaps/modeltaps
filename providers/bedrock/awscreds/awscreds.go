// Package awscreds parses the channel key of AWS-based channels (Bedrock) and provides a
// credentials provider usable by the AWS SDK.
//
// Supported key shapes:
//   - Bearer:     Region|BearerToken
//   - Static:     Region|AccessKeyID|SecretAccessKey[|SessionToken]
//   - AssumeRole: Region|AccessKeyID|SecretAccessKey|RoleArn
//     Role chain: Region|AccessKeyID|SecretAccessKey|SessionToken|RoleArn
//
// AssumeRole is the least-privilege setup AWS recommends: the long-lived AK/SK only needs
// sts:AssumeRole, and the Bedrock permissions live on the target role. The gateway calls STS at
// runtime and caches the temporary credentials per (key, proxy), refreshing them before they
// expire, so STS is not called once per request.
package awscreds

import (
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/requester"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	// roleSessionName shows up in the CloudTrail AssumeRole records, so the caller can be told
	// apart when auditing.
	roleSessionName = "modeltaps"
	// assumeRoleDuration is the lifetime of the temporary credentials. The minimum
	// MaxSessionDuration of an IAM role is one hour, so one hour is valid for every role.
	//
	// Role chaining (five segments: the source AK/SK are themselves temporary STS credentials)
	// is supported, but STS requires the target session to end no later than the source session:
	// with 30 minutes left on the source, asking for 3600s is rejected, which shows up as
	// intermittent signing failures. The duration stays fixed on purpose: shortening it by the
	// remaining time would make the cache entries drift with the source lifetime, and the
	// caller is responsible for rotating the source credentials before they run out.
	assumeRoleDuration = time.Hour
	// refreshWindow renews the credentials this long before they expire, so a request is never
	// signed with credentials that expire at that very moment.
	refreshWindow = 5 * time.Minute
)

// Config is the credential configuration parsed from a channel key. All fields are strings, so
// the struct can be compared directly and used as a cache key.
type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	// RoleArn, when set, makes the provider call STS AssumeRole with the AK/SK above and sign
	// with the temporary credentials.
	RoleArn string
	// APIToken is set in Bearer mode (a Bedrock API key); SigV4 is not used then.
	APIToken string
}

// ParseKey parses a channel key. ok is false when there are too few segments, too many
// segments, or a five-segment key whose last segment is not a role ARN.
//
// A fourth segment shaped like an IAM role ARN (see isRoleArn) is the RoleArn; otherwise it is
// the SessionToken (an STS session token never starts with "arn:").
//
// Five segments express a role chain: the source credentials are temporary STS credentials and
// are used to assume another role. The fourth segment is then the SessionToken and the fifth the
// RoleArn, both required, so the shape cannot be inferred as it is for four segments. More than
// five segments is rejected instead of silently truncated, so an extra segment is never dropped
// while the user believes the configuration took effect.
func ParseKey(key string) (cfg Config, ok bool) {
	keys := strings.Split(key, "|")
	if len(keys) < 2 || len(keys) > 5 {
		return Config{}, false
	}
	cfg.Region = keys[0]
	if len(keys) == 2 {
		cfg.APIToken = keys[1]
		return cfg, true
	}
	cfg.AccessKeyID = keys[1]
	cfg.SecretAccessKey = keys[2]

	if len(keys) == 5 {
		if !isRoleArn(keys[4]) {
			// The fifth segment is always the RoleArn. A value that is not an ARN means the
			// segments are in the wrong order; reject it instead of using the RoleArn as a
			// session token.
			return Config{}, false
		}
		cfg.SessionToken = keys[3]
		cfg.RoleArn = keys[4]
		return cfg, true
	}

	if len(keys) == 4 && keys[3] != "" {
		if isRoleArn(keys[3]) {
			cfg.RoleArn = keys[3]
		} else {
			cfg.SessionToken = keys[3]
		}
	}
	return cfg, true
}

// isRoleArn reports whether s is an IAM role ARN (aws, aws-cn and aws-us-gov partitions).
func isRoleArn(s string) bool {
	return strings.HasPrefix(s, "arn:") && strings.Contains(s, ":role/")
}

// Provider returns the credentials provider for the AWS SDK.
//   - Without RoleArn: static credentials.
//   - With RoleArn: an AssumeRole provider cached globally by (cfg, proxy). The temporary
//     credentials are renewed automatically, and concurrent first uses are coalesced into one
//     STS call by the credentials cache.
//
// proxy is the channel proxy: the STS call leaves through the same egress as the Bedrock call.
func Provider(cfg Config, proxy string) aws.CredentialsProvider {
	if cfg.RoleArn == "" {
		return aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		)
	}
	return assumeRoleCache.get(cfg, proxy)
}

// newSTSClient builds the STS client; tests replace it with a fake.
var newSTSClient = func(cfg Config, proxy string) stscreds.AssumeRoleAPIClient {
	return sts.New(sts.Options{
		Region: cfg.Region,
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		),
		HTTPClient: requester.GetHTTPClient(proxy),
	})
}

type cacheKey struct {
	cfg   Config
	proxy string
}

// providerCache caches AssumeRole providers by channel key and proxy. Changing the key of a
// channel lands on a new entry; the stale entry is tiny (a few strings and one credential) and
// is not evicted.
type providerCache struct {
	mu      sync.Mutex
	entries map[cacheKey]*aws.CredentialsCache
}

var assumeRoleCache = &providerCache{entries: map[cacheKey]*aws.CredentialsCache{}}

func (c *providerCache) get(cfg Config, proxy string) *aws.CredentialsCache {
	k := cacheKey{cfg: cfg, proxy: proxy}
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.entries[k]; ok {
		return p
	}
	p := aws.NewCredentialsCache(
		stscreds.NewAssumeRoleProvider(newSTSClient(cfg, proxy), cfg.RoleArn, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = roleSessionName
			o.Duration = assumeRoleDuration
		}),
		func(o *aws.CredentialsCacheOptions) {
			o.ExpiryWindow = refreshWindow
		},
	)
	c.entries[k] = p
	return p
}
