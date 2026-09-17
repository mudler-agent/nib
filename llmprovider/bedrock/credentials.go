package bedrock

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file resolves AWS credentials from the environment and ~/.aws/credentials.
// The chain (first hit wins):
//  1. Environment: AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY [+ AWS_SESSION_TOKEN]
//  2. Profile in ~/.aws/credentials (static keys only)
//
// SSO, credential_process, role-chaining, ECS, and IMDS are not implemented
// here — they add significant complexity and are rarely needed for CLI use.
// Users who need them can set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY env
// vars or use a credential_process wrapper.

// resolveCredentials resolves IAM credentials for SigV4 signing.
func resolveCredentials(profile string) (awsCredentials, error) {
	// 1. Environment
	if ak := os.Getenv("AWS_ACCESS_KEY_ID"); ak != "" {
		sk := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if sk != "" {
			return awsCredentials{
				AccessKeyID:     ak,
				SecretAccessKey: sk,
				SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
			}, nil
		}
	}

	// 2. Profile in ~/.aws/credentials
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
	}
	if profile == "" {
		profile = "default"
	}
	creds, ok, err := readProfileCredentials(profile)
	if err != nil {
		return awsCredentials{}, err
	}
	if ok {
		return creds, nil
	}

	return awsCredentials{}, fmt.Errorf("bedrock: unable to resolve AWS credentials — set AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY env vars or configure ~/.aws/credentials")
}

// readProfileCredentials reads static keys from ~/.aws/credentials for the
// named profile. Returns ok=false if the profile doesn't exist or has no keys.
func readProfileCredentials(profile string) (awsCredentials, bool, error) {
	credsPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return awsCredentials{}, false, nil
		}
		credsPath = filepath.Join(home, ".aws", "credentials")
	}

	entries, err := parseAWSIni(credsPath)
	if err != nil {
		return awsCredentials{}, false, err
	}
	section, ok := entries[profile]
	if !ok {
		return awsCredentials{}, false, nil
	}
	ak, sk := section["aws_access_key_id"], section["aws_secret_access_key"]
	if ak == "" || sk == "" {
		return awsCredentials{}, false, nil
	}
	return awsCredentials{
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		SessionToken:    section["aws_session_token"],
	}, true, nil
}

// resolveRegion returns the AWS region from env vars, defaulting to us-east-1.
func resolveRegion() string {
	for _, v := range []string{"AWS_REGION", "AWS_DEFAULT_REGION"} {
		if r := os.Getenv(v); r != "" {
			return r
		}
	}
	return "us-east-1"
}

// parseAWSIni parses an AWS INI file (~/.aws/credentials or ~/.aws/config)
// into a map of section name → key/value pairs. Section headers in
// ~/.aws/config use the prefix "profile " which is stripped.
func parseAWSIni(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	result := make(map[string]map[string]string)
	var current string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(line[1 : len(line)-1])
			// Strip "profile " prefix from ~/.aws/config section names.
			current = strings.TrimPrefix(current, "profile ")
			if result[current] == nil {
				result[current] = make(map[string]string)
			}
			continue
		}
		if current == "" {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		result[current][key] = val
	}
	return result, scanner.Err()
}
