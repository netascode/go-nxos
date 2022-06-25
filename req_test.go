package nxos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

// TestSetRaw tests the Body::SetRaw method.
func TestSetRaw(t *testing.T) {
	name := Body{}.SetRaw("a", `{"name":"a"}`).Res().Get("a.name").Str
	assert.Equal(t, "a", name)
}

// TestDelete tests the Body::Delete method.
func TestDelete(t *testing.T) {
	body := Body{}
	body = body.SetRaw("a", `{"name":"a"}`)
	assert.Equal(t, "a", body.Res().Get("a.name").Str)
	body = body.Delete("a.name")
	assert.Equal(t, "", body.Res().Get("a.name").Str)
}

// TestQuery tests the Query function.
func TestQuery(t *testing.T) {
	defer gock.Off()
	client := testClient()

	gock.New(testURL).Get("/url").MatchParam("foo", "bar").Reply(200)
	_, err := client.Get("/url", Query("foo", "bar"))
	assert.NoError(t, err)

	// Test case for comma-separated parameters
	gock.New(testURL).Get("/url").MatchParam("foo", "bar,baz").Reply(200)
	_, err = client.Get("/url", Query("foo", "bar,baz"))
	assert.NoError(t, err)
}
