package filesystem

import (
	"fmt"
	"strings"
)

// Settings keys consumed only by the s3 protocol (region and addressing travel in the root's options JSON, see optionKeys).
const (
	KeyRegion     = "region"
	KeyAddressing = "addressing" // "path" (default) or "virtual"
)

// RegisterS3Protocol makes "s3" a creatable storage protocol (factory, SupportedProtocols, generic scanner). It is explicit, not an init(), so
// a binary or test that does not want s3 is unchanged; main() calls it once at start-up. Safe to call more than once.
func RegisterS3Protocol() error {
	if IsRegisteredProtocol("s3") {
		return nil
	}
	return RegisterProtocol(ProtocolSpec{
		Name: "s3",
		// url = endpoint, path = "bucket[/prefix]", username = access key, password = secret key.
		Keys: map[string]KeyType{
			KeyURL: KeyString, KeyPath: KeyString, KeyUsername: KeyString, KeyPassword: KeyString,
			KeyRegion: KeyString, KeyAddressing: KeyString,
		},
		Required: []string{KeyURL, KeyPath},
		New:      newS3FromSettings,
	})
}

// ParseS3Path splits the "path" setting into bucket and prefix.
func ParseS3Path(p string) (bucket, prefix string, err error) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return "", "", fmt.Errorf("s3: path must be \"bucket[/prefix]\"")
	}
	bucket, prefix, _ = strings.Cut(p, "/")
	return bucket, prefix, nil
}

func newS3FromSettings(settings map[string]interface{}) (FileSystemClient, error) {
	bucket, prefix, err := ParseS3Path(StringSetting(settings, KeyPath, ""))
	if err != nil {
		return nil, err
	}
	pathStyle := true
	switch a := StringSetting(settings, KeyAddressing, "path"); a {
	case "", "path":
	case "virtual":
		pathStyle = false
	default:
		return nil, fmt.Errorf("s3: addressing must be \"path\" or \"virtual\"")
	}
	return NewS3Client(&S3Config{
		Endpoint:  StringSetting(settings, KeyURL, ""),
		Region:    StringSetting(settings, KeyRegion, ""),
		Bucket:    bucket,
		Prefix:    prefix,
		AccessKey: StringSetting(settings, KeyUsername, ""),
		SecretKey: StringSetting(settings, KeyPassword, ""),
		PathStyle: pathStyle,
	})
}
