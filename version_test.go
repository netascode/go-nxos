package nxos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

// TestParseVersion tests the ParseVersion function.
func TestParseVersion(t *testing.T) {
	cases := []struct {
		in    string
		major int
		minor int
		maint int
		rb    string
	}{
		{"10.6(4)", 10, 6, 4, ""},
		{"10.3(4a)", 10, 3, 4, "a"},
		{"10.2(3t)", 10, 2, 3, "t"},
		{"9.3(10)", 9, 3, 10, ""},
		{"7.0(3)I7(9)", 7, 0, 3, ""},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.in)
		assert.NoError(t, err, c.in)
		assert.Equal(t, c.major, v.Major, c.in)
		assert.Equal(t, c.minor, v.Minor, c.in)
		assert.Equal(t, c.maint, v.Maintenance, c.in)
		assert.Equal(t, c.rb, v.Rebuild, c.in)
		assert.Equal(t, c.in, v.String(), c.in)
	}

	for _, in := range []string{"", "10.6", "10.6.4", "abc"} {
		_, err := ParseVersion(in)
		assert.Error(t, err, in)
	}

	assert.Panics(t, func() { MustParseVersion("abc") })
	assert.Equal(t, "10.6(1)", Version{Major: 10, Minor: 6, Maintenance: 1}.String())
}

// TestVersionCompare tests the Version::Compare and Version::AtLeast methods.
func TestVersionCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"10.6(4)", "10.6(4)", 0},
		{"10.6(4)", "10.6(1)", 1},
		{"10.5(6)", "10.6(1)", -1},
		{"9.3(10)", "10.1(1)", -1},
		{"10.6(10)", "10.6(9)", 1},
		{"10.3(4a)", "10.3(4)", 0},
	}
	for _, c := range cases {
		a, b := MustParseVersion(c.a), MustParseVersion(c.b)
		assert.Equal(t, c.want, a.Compare(b), c.a+" vs "+c.b)
		assert.Equal(t, c.want >= 0, a.AtLeast(b), c.a+" vs "+c.b)
	}
}

// TestClientVersion tests the Client::Version method.
func TestClientVersion(t *testing.T) {
	defer gock.Off()
	client := testClient()

	// Failed request is not cached
	gock.New(testURL).Get("/api/mo/sys/showversion.json").Reply(500)
	_, err := client.Version()
	assert.Error(t, err)

	// Missing version attribute
	gock.New(testURL).Get("/api/mo/sys/showversion.json").Reply(200).BodyString(`{"imdata":[]}`)
	_, err = client.Version()
	assert.Error(t, err)

	// Unparsable version
	gock.New(testURL).
		Get("/api/mo/sys/showversion.json").
		Reply(200).
		BodyString(`{"imdata":[{"sysmgrShowVersion":{"attributes":{"nxosVersion":"abc"}}}]}`)
	_, err = client.Version()
	assert.Error(t, err)

	// Successful retrieval
	gock.New(testURL).
		Get("/api/mo/sys/showversion.json").
		Reply(200).
		BodyString(`{"imdata":[{"sysmgrShowVersion":{"attributes":{"nxosVersion":"10.6(4)"}}}]}`)
	v, err := client.Version()
	assert.NoError(t, err)
	assert.Equal(t, MustParseVersion("10.6(4)"), v)

	// Cached, no further request
	v, err = client.Version()
	assert.NoError(t, err)
	assert.Equal(t, "10.6(4)", v.String())
	assert.True(t, gock.IsDone())
}
