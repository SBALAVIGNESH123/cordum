package gateway

import "regexp"

// packTopicNamespaceViolationRE matches the error text produced by
// ValidatePackManifest / validateTimeoutsPatch / ValidatePoolsPatch when a
// topic is not namespaced under the pack's declared metadata.id. The capture
// groups are (1) the offending topic name and (2) the derived pack id. The
// prefix variants — "topic", "pools topic", "timeouts topic" — are folded
// into the alternation so a single classifier covers all three validator
// sites without sentinel error types (which would couple the pure validator
// package to logging concerns).
var packTopicNamespaceViolationRE = regexp.MustCompile(
	`^(?:topic|pools topic|timeouts topic) "([^"]+)" must be namespaced under job\.[^.]+\.\* \(derived from pack metadata\.id="([^"]+)"\)$`,
)

// classifyPackValidationError inspects a ValidatePackManifest /
// ValidatePoolsPatch / ValidateTimeoutsPatch error and, when the error is a
// topic-namespace violation, returns the offending topic, the derived pack
// id, and ok=true. For any other validator error (missing schema, invalid
// metadata.id, etc.) it returns ok=false and the caller should log a generic
// pack-install-failed entry instead.
//
// Regex-extraction is preferred over sentinel error types because the pure
// `packs` validator package has no logging dependencies today and adding a
// typed error would leak that concern upward. The error text is already the
// stable surface (pinned by TestValidatePackManifest_TopicNamespaceReject).
func classifyPackValidationError(err error) (topic, packID string, ok bool) {
	if err == nil {
		return "", "", false
	}
	match := packTopicNamespaceViolationRE.FindStringSubmatch(err.Error())
	if len(match) != 3 {
		return "", "", false
	}
	return match[1], match[2], true
}
