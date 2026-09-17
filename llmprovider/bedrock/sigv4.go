// Package bedrock adapts the Amazon Bedrock Converse Stream API to cogito.LLM.
//
// It talks directly to bedrock-runtime.{region}.amazonaws.com with AWS SigV4
// signing and decodes the application/vnd.amazon.eventstream response — no
// AWS SDK dependency. Auth is either a Bedrock bearer token (Authorization:
// Bearer) or IAM credentials resolved from the environment / ~/.aws/credentials.
package bedrock

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"time"
)

// awsCredentials is a set of IAM credentials for SigV4 signing.
type awsCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // optional; set for STS/role/SO credentials
}

// signParams holds the inputs for SigV4 request signing.
type signParams struct {
	Method  string
	Host    string // hostname only
	Path    string // URI path, e.g. /model/anthropic.claude/converse-stream
	Query   string // pre-built query string without leading ?
	Headers map[string]string
	Body    []byte
	Region  string
	Service string
	Creds   awsCredentials
	Time    time.Time
}

// signedHeaders is the set of headers SigV4 produces.
type signedHeaders struct {
	Host             string
	AmzDate          string
	AmzContentSHA256 string
	Authorization    string
	AmzSecurityToken string // empty when no session token
}

const (
	sigAlgorithm = "AWS4-HMAC-SHA256"
	sigKeyType   = "aws4_request"
)

// Headers that are never included in the signature. Lowercased.
var sigUnsignable = map[string]bool{
	"authorization":     true,
	"cache-control":     true,
	"connection":        true,
	"expect":            true,
	"from":              true,
	"keep-alive":        true,
	"max-forwards":     true,
	"pragma":            true,
	"referer":           true,
	"te":                true,
	"trailer":           true,
	"transfer-encoding": true,
	"upgrade":           true,
	"user-agent":        true,
	"x-amzn-trace-id":   true,
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// getSigningKey derives the SigV4 signing key via the HMAC chain
// kSecret → kDate → kRegion → kService → kSigning.
func getSigningKey(secretAccessKey, shortDate, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(shortDate))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte(sigKeyType))
}

// formatAmzDate returns the long (yyyyMMddTHHmmssZ) and short (yyyyMMdd)
// forms used by SigV4.
func formatAmzDate(t time.Time) (longDate, shortDate string) {
	utc := t.UTC()
	longDate = utc.Format("20060102T150405Z")
	shortDate = utc.Format("20060102")
	return
}

// canonicalPath percent-encodes each path segment per RFC 3986 while keeping
// "/" literal — matching the smithy default (uriEscapePath: true, then revert
// the double-encoding of "/").
func canonicalPath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		segments[i] = encodeRfc3986(seg)
	}
	return strings.Join(segments, "/")
}

func encodeRfc3986(s string) string {
	encoded := url.QueryEscape(s)
	// url.QueryEscape encodes '!' as "%21" etc. — RFC 3986 unreserved chars
	// stay literal, but the AWS canonical form uppercases hex and escapes
	// !'()* which Go's QueryEscape leaves alone.
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	for _, c := range "!'()*" {
		encoded = strings.ReplaceAll(encoded, string(c), "%"+strings.ToUpper(string(c)))
	}
	return encoded
}

// canonicalQuery sorts and encodes query-string pairs per the SigV4 spec.
func canonicalQuery(query string) string {
	if query == "" {
		return ""
	}
	type pair struct{ k, v string }
	var pairs []pair
	for _, part := range strings.Split(query, "&") {
		if part == "" {
			continue
		}
		eq := strings.Index(part, "=")
		var rawKey, rawValue string
		if eq < 0 {
			rawKey = part
		} else {
			rawKey = part[:eq]
			rawValue = part[eq+1:]
		}
		decKey, _ := url.QueryUnescape(rawKey)
		decVal, _ := url.QueryUnescape(rawValue)
		pairs = append(pairs, pair{encodeRfc3986(decKey), encodeRfc3986(decVal)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.k + "=" + p.v
	}
	return strings.Join(out, "&")
}

// signRequest computes the SigV4 signature and returns the headers to attach.
func signRequest(params signParams) signedHeaders {
	creds := params.Creds
	now := params.Time
	if now.IsZero() {
		now = time.Now()
	}
	longDate, shortDate := formatAmzDate(now)
	payloadHash := sha256Hex(params.Body)

	// Assemble headers to sign.
	signed := map[string]string{
		"host":                 params.Host,
		"x-amz-date":           longDate,
		"x-amz-content-sha256": payloadHash,
	}
	if creds.SessionToken != "" {
		signed["x-amz-security-token"] = creds.SessionToken
	}
	for k, v := range params.Headers {
		lk := strings.ToLower(k)
		if sigUnsignable[lk] || strings.HasPrefix(lk, "proxy-") || strings.HasPrefix(lk, "sec-") {
			continue
		}
		signed[lk] = strings.TrimSpace(v)
	}

	sortedNames := make([]string, 0, len(signed))
	for k := range signed {
		sortedNames = append(sortedNames, k)
	}
	sort.Strings(sortedNames)

	var canonicalHeadersBuilder strings.Builder
	for i, n := range sortedNames {
		if i > 0 {
			canonicalHeadersBuilder.WriteByte('\n')
		}
		canonicalHeadersBuilder.WriteString(n)
		canonicalHeadersBuilder.WriteByte(':')
		canonicalHeadersBuilder.WriteString(signed[n])
	}
	canonicalHeadersBuilder.WriteByte('\n')
	signedHeadersStr := strings.Join(sortedNames, ";")

	canonicalRequest := strings.Join([]string{
		strings.ToUpper(params.Method),
		canonicalPath(params.Path),
		canonicalQuery(params.Query),
		canonicalHeadersBuilder.String(),
		signedHeadersStr,
		payloadHash,
	}, "\n")

	scope := shortDate + "/" + params.Region + "/" + params.Service + "/" + sigKeyType
	stringToSign := strings.Join([]string{
		sigAlgorithm,
		longDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := getSigningKey(creds.SecretAccessKey, shortDate, params.Region, params.Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authorization := sigAlgorithm + " Credential=" + creds.AccessKeyID + "/" + scope +
		", SignedHeaders=" + signedHeadersStr + ", Signature=" + signature

	out := signedHeaders{
		Host:             params.Host,
		AmzDate:          longDate,
		AmzContentSHA256: payloadHash,
		Authorization:    authorization,
	}
	if creds.SessionToken != "" {
		out.AmzSecurityToken = creds.SessionToken
	}
	return out
}
