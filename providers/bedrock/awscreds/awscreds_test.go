package awscreds

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
)

const testRoleArn = "arn:aws:iam::123456789012:role/bedrock-invoke"

func TestParseKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want Config
		ok   bool
	}{
		{"bearer", "us-east-1|tok", Config{Region: "us-east-1", APIToken: "tok"}, true},
		{"static", "us-east-1|AK|SK", Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK"}, true},
		{"static with empty session token", "us-east-1|AK|SK|", Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK"}, true},
		{"static with session token", "us-east-1|AK|SK|ST", Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", SessionToken: "ST"}, true},
		{"assume role", "us-east-1|AK|SK|" + testRoleArn, Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", RoleArn: testRoleArn}, true},
		{"assume role in china partition", "cn-north-1|AK|SK|arn:aws-cn:iam::123456789012:role/r", Config{Region: "cn-north-1", AccessKeyID: "AK", SecretAccessKey: "SK", RoleArn: "arn:aws-cn:iam::123456789012:role/r"}, true},
		{"role chain", "us-east-1|AK|SK|ST|" + testRoleArn, Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", SessionToken: "ST", RoleArn: testRoleArn}, true},
		{"too few segments", "us-east-1", Config{}, false},
		{"empty", "", Config{}, false},
		{"fifth segment is not a role arn", "us-east-1|AK|SK|" + testRoleArn + "|ST", Config{}, false},
		{"too many segments", "us-east-1|AK|SK|ST|" + testRoleArn + "|extra", Config{}, false},
		{"fourth segment arn that is not a role stays a session token", "us-east-1|AK|SK|arn:aws:iam::123456789012:user/u", Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", SessionToken: "arn:aws:iam::123456789012:user/u"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseKey(tc.key)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("config = %+v, want %+v", got, tc.want)
			}
		})
	}
}

type fakeSTS struct {
	calls atomic.Int32
	// expires is how long each issued credential lives.
	expires time.Duration
	input   atomic.Pointer[sts.AssumeRoleInput]
}

func (f *fakeSTS) AssumeRole(_ context.Context, in *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	n := f.calls.Add(1)
	f.input.Store(in)
	exp := time.Now().Add(f.expires)
	return &sts.AssumeRoleOutput{Credentials: &ststypes.Credentials{
		AccessKeyId:     aws.String("TEMPAK"),
		SecretAccessKey: aws.String("TEMPSK"),
		SessionToken:    aws.String("TEMPST" + string(rune('0'+n))),
		Expiration:      &exp,
	}}, nil
}

// withFakeSTS swaps the STS client constructor and starts from an empty cache.
func withFakeSTS(t *testing.T, fake *fakeSTS) {
	t.Helper()
	oldNew, oldCache := newSTSClient, assumeRoleCache
	newSTSClient = func(Config, string) stscreds.AssumeRoleAPIClient { return fake }
	assumeRoleCache = &providerCache{entries: map[cacheKey]*aws.CredentialsCache{}}
	t.Cleanup(func() { newSTSClient, assumeRoleCache = oldNew, oldCache })
}

func TestProviderStaticCredentials(t *testing.T) {
	cfg := Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", SessionToken: "ST"}
	creds, err := Provider(cfg, "").Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessKeyID != "AK" || creds.SecretAccessKey != "SK" || creds.SessionToken != "ST" {
		t.Fatalf("unexpected static credentials: %+v", creds)
	}
}

func TestProviderAssumeRoleCachesTemporaryCredentials(t *testing.T) {
	fake := &fakeSTS{expires: time.Hour}
	withFakeSTS(t, fake)
	cfg := Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", RoleArn: testRoleArn}

	for i := 0; i < 5; i++ {
		creds, err := Provider(cfg, "").Retrieve(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if creds.AccessKeyID != "TEMPAK" || creds.SessionToken != "TEMPST1" {
			t.Fatalf("expected the cached temporary credentials, got %+v", creds)
		}
	}
	if got := fake.calls.Load(); got != 1 {
		t.Fatalf("STS called %d times, want 1", got)
	}
	in := fake.input.Load()
	if aws.ToString(in.RoleArn) != testRoleArn || aws.ToString(in.RoleSessionName) != roleSessionName {
		t.Fatalf("unexpected AssumeRole input: role=%s session=%s", aws.ToString(in.RoleArn), aws.ToString(in.RoleSessionName))
	}
	if aws.ToInt32(in.DurationSeconds) != int32(assumeRoleDuration/time.Second) {
		t.Fatalf("unexpected duration: %d", aws.ToInt32(in.DurationSeconds))
	}
}

func TestProviderAssumeRoleRefreshesBeforeExpiry(t *testing.T) {
	// Credentials that live shorter than the refresh window are renewed on every use.
	fake := &fakeSTS{expires: refreshWindow / 2}
	withFakeSTS(t, fake)
	cfg := Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", RoleArn: testRoleArn}

	first, err := Provider(cfg, "").Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Provider(cfg, "").Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls.Load() != 2 || first.SessionToken == second.SessionToken {
		t.Fatalf("expected a refresh inside the window, calls=%d first=%s second=%s", fake.calls.Load(), first.SessionToken, second.SessionToken)
	}
}

func TestProviderCacheIsKeyedByConfigAndProxy(t *testing.T) {
	fake := &fakeSTS{expires: time.Hour}
	withFakeSTS(t, fake)
	cfg := Config{Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK", RoleArn: testRoleArn}
	other := cfg
	other.RoleArn = "arn:aws:iam::123456789012:role/other"

	for _, p := range []aws.CredentialsProvider{
		Provider(cfg, ""), Provider(cfg, "http://proxy-a:8080"), Provider(other, ""), Provider(cfg, ""),
	} {
		if _, err := p.Retrieve(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// (cfg, "") is shared by the first and last provider; the other two keys are separate entries.
	if got := fake.calls.Load(); got != 3 {
		t.Fatalf("STS called %d times, want 3", got)
	}
}
