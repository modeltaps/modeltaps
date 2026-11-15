package preset

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// volcBilling is the billing OpenAPI of Volcengine (or BytePlus): QueryBalanceAcct, signed with
// the account's AccessKey pair (HMAC-SHA256, like AWS SigV4 but with "request" as the scope
// terminator and the secret used as is). The Ark API key cannot read the balance.
// https://www.volcengine.com/docs/6369/67268 (signing), billing-center QueryBalanceAcct (action).
type volcBilling struct {
	Host   string
	Region string
}

var volcengineBilling = volcBilling{Host: "open.volcengineapi.com", Region: "cn-north-1"}

// volcNow and volcHTTP are replaced in tests.
var (
	volcNow  = time.Now
	volcHTTP = func(req *http.Request) (*http.Response, error) { return http.DefaultClient.Do(req) }
)

type queryBalanceAcctResponse struct {
	ResponseMetadata struct {
		Error *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"ResponseMetadata"`
	Result struct {
		AvailableBalance string `json:"AvailableBalance"`
	} `json:"Result"`
}

// balance reads AvailableBalance (CNY for Volcengine) with the key's AccessKey pair, given as
// the second and third parts of the channel key.
func (b volcBilling) balance(p *Provider) (float64, error) {
	if len(p.extraKeys) < 2 || p.extraKeys[0] == "" || p.extraKeys[1] == "" {
		return 0, errors.New("the balance needs the account's AccessKey: enter the key as apiKey|AccessKeyID|SecretAccessKey")
	}
	req, err := b.request(p.extraKeys[0], p.extraKeys[1], volcNow().UTC())
	if err != nil {
		return 0, err
	}
	resp, err := volcHTTP(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	var out queryBalanceAcctResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("billing API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 200)])))
	}
	if e := out.ResponseMetadata.Error; e != nil {
		return 0, fmt.Errorf("billing API: %s %s", e.Code, e.Message)
	}
	balance, err := strconv.ParseFloat(out.Result.AvailableBalance, 64)
	if err != nil {
		return 0, errors.New("billing API returned no AvailableBalance")
	}
	return balance, nil
}

// request builds the signed QueryBalanceAcct call.
func (b volcBilling) request(accessKey, secretKey string, now time.Time) (*http.Request, error) {
	const service, action, version = "billing", "QueryBalanceAcct", "2022-01-01"
	body := []byte("{}")
	query := url.Values{"Action": {action}, "Version": {version}}.Encode()
	req, err := http.NewRequest(http.MethodPost, "https://"+b.Host+"/?"+query, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	xDate := now.Format("20060102T150405Z")
	day := xDate[:8]
	bodyHash := sha256Hex(body)
	contentType := "application/json; charset=utf-8"
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Date", xDate)
	req.Header.Set("X-Content-Sha256", bodyHash)

	signedHeaders := "content-type;host;x-content-sha256;x-date"
	canonical := strings.Join([]string{
		http.MethodPost,
		"/",
		query,
		"content-type:" + contentType + "\nhost:" + b.Host + "\nx-content-sha256:" + bodyHash + "\nx-date:" + xDate + "\n",
		signedHeaders,
		bodyHash,
	}, "\n")
	scope := day + "/" + b.Region + "/" + service + "/request"
	stringToSign := "HMAC-SHA256\n" + xDate + "\n" + scope + "\n" + sha256Hex([]byte(canonical))
	key := hmacSHA256([]byte(secretKey), day)
	for _, part := range []string{b.Region, service, "request"} {
		key = hmacSHA256(key, part)
	}
	signature := hex.EncodeToString(hmacSHA256(key, stringToSign))
	req.Header.Set("Authorization", "HMAC-SHA256 Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
	return req, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
