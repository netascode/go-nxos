package nxos

import (
	"fmt"
	"regexp"
	"strconv"
)

var versionRegex = regexp.MustCompile(`^(\d+)\.(\d+)\((\d+)([a-zA-Z]*)\)`)

// Version is a parsed NX-OS software version, e.g. "10.6(4)" or "10.3(4a)".
type Version struct {
	// Major is the major release number, e.g. 10 in "10.6(4)".
	Major int
	// Minor is the minor release number, e.g. 6 in "10.6(4)".
	Minor int
	// Maintenance is the maintenance release number, e.g. 4 in "10.6(4)".
	Maintenance int
	// Rebuild is the optional rebuild suffix, e.g. "a" in "10.3(4a)".
	// It is not considered when comparing versions.
	Rebuild string
	// Raw is the original version string.
	Raw string
}

// ParseVersion parses an NX-OS version string such as "10.6(4)", "10.3(4a)" or "9.3(10)".
// Trailing train information (e.g. "I7(9)" in "7.0(3)I7(9)") is ignored.
func ParseVersion(s string) (Version, error) {
	m := versionRegex.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid NX-OS version: %q", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	maintenance, _ := strconv.Atoi(m[3])
	return Version{
		Major:       major,
		Minor:       minor,
		Maintenance: maintenance,
		Rebuild:     m[4],
		Raw:         s,
	}, nil
}

// MustParseVersion is like ParseVersion but panics if the version cannot be parsed.
func MustParseVersion(s string) Version {
	v, err := ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Compare returns -1, 0 or 1 if v is lower than, equal to or higher than other.
// Only major, minor and maintenance numbers are compared.
func (v Version) Compare(other Version) int {
	for _, d := range [3]int{v.Major - other.Major, v.Minor - other.Minor, v.Maintenance - other.Maintenance} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	return 0
}

// AtLeast returns true if v is equal to or higher than other.
func (v Version) AtLeast(other Version) bool {
	return v.Compare(other) >= 0
}

// String returns the original version string, or a normalized one if not available.
func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	return fmt.Sprintf("%d.%d(%d%s)", v.Major, v.Minor, v.Maintenance, v.Rebuild)
}

// Version returns the NX-OS software version of the device.
// The version is retrieved from "sys/showversion" on first use and cached for the lifetime of the client.
// Failed attempts are not cached, so a subsequent call will retry.
func (client *Client) Version() (Version, error) {
	client.versionMutex.Lock()
	defer client.versionMutex.Unlock()
	if client.version != nil {
		return *client.version, nil
	}
	res, err := client.GetDn("sys/showversion")
	if err != nil {
		return Version{}, err
	}
	raw := res.Get("sysmgrShowVersion.attributes.nxosVersion").Str
	if raw == "" {
		return Version{}, fmt.Errorf("failed to retrieve NX-OS version from sys/showversion")
	}
	v, err := ParseVersion(raw)
	if err != nil {
		return Version{}, err
	}
	client.version = &v
	return v, nil
}
