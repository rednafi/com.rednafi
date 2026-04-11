package site_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHomepageSchemaCompleteness verifies the JSON-LD schema on the homepage
// includes both WebSite and Person objects with all required fields.
// Incomplete schema.org data causes rich result eligibility to drop.
func TestHomepageSchemaCompleteness(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	jsonLd, err := page.Locator(`script[type="application/ld+json"]`).TextContent()
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonLd), &schema))
	assert.Equal(t, "https://schema.org", schema["@context"])

	graph, ok := schema["@graph"].([]any)
	require.True(t, ok, "@graph should be an array")
	require.Len(t, graph, 2, "@graph should have WebSite and Person")

	var website, person map[string]any
	for _, item := range graph {
		m, _ := item.(map[string]any)
		switch m["@type"] {
		case "WebSite":
			website = m
		case "Person":
			person = m
		}
	}

	t.Run("WebSite has required fields", func(t *testing.T) {
		require.NotNil(t, website, "WebSite object missing")
		assert.Equal(t, "Redowan's Reflections", website["name"])
		assert.NotEmpty(t, website["url"])
		assert.NotEmpty(t, website["description"])
	})

	t.Run("Person has jobTitle", func(t *testing.T) {
		require.NotNil(t, person, "Person object missing")
		assert.Equal(t, "Software Engineer", person["jobTitle"])
	})

	t.Run("Person has sameAs social links", func(t *testing.T) {
		require.NotNil(t, person)
		sameAs, ok := person["sameAs"].([]any)
		require.True(t, ok, "sameAs should be an array")
		assert.GreaterOrEqual(t, len(sameAs), 3, "should have GitHub, LinkedIn, Bluesky")
	})

	t.Run("Person has name and URL", func(t *testing.T) {
		require.NotNil(t, person)
		assert.NotEmpty(t, person["name"])
		assert.NotEmpty(t, person["url"])
	})
}

// TestArticleSchemaCompleteness verifies BlogPosting JSON-LD includes all
// fields needed for Google's article rich results.
func TestArticleSchemaCompleteness(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	jsonLd, err := page.Locator(`script[type="application/ld+json"]`).TextContent()
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonLd), &schema))

	t.Run("is BlogPosting type", func(t *testing.T) {
		assert.Equal(t, "BlogPosting", schema["@type"])
	})

	t.Run("has headline", func(t *testing.T) {
		assert.NotEmpty(t, schema["headline"])
	})

	t.Run("has keywords", func(t *testing.T) {
		keywords, ok := schema["keywords"].([]any)
		require.True(t, ok, "keywords should be an array")
		assert.Greater(t, len(keywords), 0)
	})

	t.Run("has mainEntityOfPage", func(t *testing.T) {
		entity, ok := schema["mainEntityOfPage"].(map[string]any)
		require.True(t, ok, "mainEntityOfPage should be an object")
		assert.Equal(t, "WebPage", entity["@type"])
		assert.NotEmpty(t, entity["@id"])
	})

	t.Run("has author with type and name", func(t *testing.T) {
		author, ok := schema["author"].(map[string]any)
		require.True(t, ok, "author should be an object")
		assert.Equal(t, "Person", author["@type"])
		assert.NotEmpty(t, author["name"])
	})

	t.Run("has publisher", func(t *testing.T) {
		publisher, ok := schema["publisher"].(map[string]any)
		require.True(t, ok, "publisher should be an object")
		assert.Equal(t, "Person", publisher["@type"])
		assert.NotEmpty(t, publisher["name"])
	})

	t.Run("has datePublished and dateModified", func(t *testing.T) {
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, schema["datePublished"])
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, schema["dateModified"])
	})
}

// TestOGImageDimensions verifies og:image includes width and height meta,
// which are required for proper social media card previews.
func TestOGImageDimensions(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("has og:image", func(t *testing.T) {
		src, err := page.Locator(`meta[property="og:image"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.NotEmpty(t, src)
	})

	t.Run("has og:image:width", func(t *testing.T) {
		width, err := page.Locator(`meta[property="og:image:width"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "1200", width)
	})

	t.Run("has og:image:height", func(t *testing.T) {
		height, err := page.Locator(`meta[property="og:image:height"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "630", height)
	})
}
