package bedrock

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

func newKeyTestProvider(key string) *BedrockProvider {
	return BedrockProviderFactory{}.Create(&model.Channel{Name: "bedrock-test", Key: key}).(*BedrockProvider)
}

func TestKeyConfigAssumeRole(t *testing.T) {
	const arn = "arn:aws:iam::123456789012:role/bedrock-invoke"
	p := newKeyTestProvider("us-west-2|AK|SK|" + arn)
	if p.Region != "us-west-2" || p.AccessKeyID != "AK" || p.SecretAccessKey != "SK" || p.RoleArn != arn || p.SessionToken != "" || p.APIToken != "" {
		t.Fatalf("unexpected provider fields: %+v", p)
	}
	cfg := p.credsConfig()
	if cfg.RoleArn != arn || cfg.AccessKeyID != "AK" || cfg.Region != "us-west-2" {
		t.Fatalf("unexpected credentials config: %+v", cfg)
	}
}

func TestKeyConfigRoleChain(t *testing.T) {
	const arn = "arn:aws:iam::123456789012:role/bedrock-invoke"
	p := newKeyTestProvider("us-west-2|AK|SK|ST|" + arn)
	if p.SessionToken != "ST" || p.RoleArn != arn {
		t.Fatalf("expected session token and role arn, got %+v", p)
	}
}

func TestKeyConfigKeepsExistingFormats(t *testing.T) {
	bearer := newKeyTestProvider("us-east-1|bedrock-api-key")
	if bearer.Region != "us-east-1" || bearer.APIToken != "bedrock-api-key" || bearer.AccessKeyID != "" || bearer.RoleArn != "" {
		t.Fatalf("unexpected bearer fields: %+v", bearer)
	}
	static := newKeyTestProvider("us-east-1|AK|SK|ST")
	if static.SessionToken != "ST" || static.RoleArn != "" {
		t.Fatalf("unexpected static fields: %+v", static)
	}
}

func TestKeyConfigRejectsMalformedKey(t *testing.T) {
	for _, key := range []string{"", "us-east-1", "us-east-1|AK|SK|ST|not-an-arn", "a|b|c|d|arn:aws:iam::1:role/r|extra"} {
		p := newKeyTestProvider(key)
		if p.Region != "" || p.AccessKeyID != "" || p.APIToken != "" || p.RoleArn != "" {
			t.Fatalf("key %q must leave the provider unconfigured, got %+v", key, p)
		}
	}
}
