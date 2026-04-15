package semver

import (
	"fmt"
	"strconv"
	"strings"
)

type Semver struct {
	Major int32
	Minor int32
	Patch int32
}

func Parse(s string) (Semver, error) {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Semver{}, fmt.Errorf("invalid semver %q: expected major.minor.patch", s)
	}
	major, err := parseNonNegative(parts[0])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid major in %q: %w", s, err)
	}
	minor, err := parseNonNegative(parts[1])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid minor in %q: %w", s, err)
	}
	patch, err := parseNonNegative(parts[2])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid patch in %q: %w", s, err)
	}
	return Semver{Major: major, Minor: minor, Patch: patch}, nil
}

func (sv Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", sv.Major, sv.Minor, sv.Patch)
}

func parseNonNegative(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("must be a non-negative integer, got %q", s)
	}
	return int32(n), nil
}
