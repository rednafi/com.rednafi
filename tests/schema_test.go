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
	require.Len(t, graph, 3, "@graph should have WebSite, Person, and recent writings ItemList")

	var website, person, itemList map[string]any
	for _, item := range graph {
		m, _ := item.(map[string]any)
		switch m["@type"] {
		case "WebSite":
			website = m
		case "Person":
			person = m
		case "ItemList":
			itemList = m
		}
	}

	t.Run("WebSite has required fields", func(t *testing.T) {
		require.NotNil(t, website, "WebSite object missing")
		assert.Equal(t, "Redowan's Reflections", website["name"])
		assert.NotEmpty(t, website["url"])
		assert.NotEmpty(t, website["description"])
		mainEntity, ok := website["mainEntity"].(map[string]any)
		require.True(t, ok, "WebSite mainEntity should point to recent writings")
		assert.Contains(t, mainEntity["@id"], "#recent-writings")
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
		assert.Equal(t, "https://rednafi.com/", person["url"])
		assert.Equal(t, "https://rednafi.com/", person["mainEntityOfPage"])
	})

	t.Run("ItemList contains recent writings", func(t *testing.T) {
		require.NotNil(t, itemList, "ItemList object missing")
		assert.Equal(t, "Recent writings", itemList["name"])
		items, ok := itemList["itemListElement"].([]any)
		require.True(t, ok, "itemListElement should be an array")
		assert.GreaterOrEqual(t, len(items), 5, "should expose recent writings")
		first, _ := items[0].(map[string]any)
		assert.Equal(t, float64(1), first["position"])
		firstItem, ok := first["item"].(map[string]any)
		require.True(t, ok, "ListItem should include an item object")
		assert.Equal(t, "BlogPosting", firstItem["@type"])
		assert.NotEmpty(t, firstItem["headline"])
		assert.NotEmpty(t, firstItem["url"])
	})

	t.Run("ItemList matches visible recent writings", func(t *testing.T) {
		require.NotNil(t, itemList, "ItemList object missing")
		items, ok := itemList["itemListElement"].([]any)
		require.True(t, ok, "itemListElement should be an array")

		visibleRaw, err := page.Locator(`.article-list .post-list .post > a`).EvaluateAll(
			`els => els.slice(0, 10).map(e => e.href)`,
		)
		require.NoError(t, err)
		visibleURLs := toStringSlice(visibleRaw)
		require.GreaterOrEqual(t, len(visibleURLs), len(items))

		schemaURLs := make([]string, 0, len(items))
		for _, item := range items {
			listItem, ok := item.(map[string]any)
			require.True(t, ok, "ListItem should be an object")
			post, ok := listItem["item"].(map[string]any)
			require.True(t, ok, "ListItem item should be an object")
			url, ok := post["url"].(string)
			require.True(t, ok, "ListItem item should include url")
			schemaURLs = append(schemaURLs, url)
		}
		assert.Equal(t, visibleURLs[:len(schemaURLs)], schemaURLs,
			"homepage ItemList JSON-LD should match the visible recent writings")
	})
}

// TestIdentityLinks verifies rel=author and rel=me links are present
// for Google Knowledge Panel and social profile verification.
func TestIdentityLinks(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("has rel=author pointing to homepage", func(t *testing.T) {
		href, err := page.Locator(`link[rel="author"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Equal(t, "https://rednafi.com/", href)
	})

	t.Run("has rel=me links for social profiles", func(t *testing.T) {
		count, err := page.Locator(`link[rel="me"]`).Count()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 3, "should have GitHub, LinkedIn, Bluesky")
	})
}

// TestArticleSchemaCompleteness verifies BlogPosting JSON-LD includes all
// fields needed for Google's article rich results.
func TestArticleSchemaCompleteness(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	jsonLd, err := page.Locator(`script[type="application/ld+json"]`).First().TextContent()
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

	t.Run("has image", func(t *testing.T) {
		images, ok := schema["image"].([]any)
		require.True(t, ok, "image should be an array")
		assert.Greater(t, len(images), 0)
	})
}

// TestBreadcrumbSchema verifies BreadcrumbList JSON-LD on article pages
// includes Home, section, and article title for Google rich snippets.
func TestBreadcrumbSchema(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	scripts := page.Locator(`script[type="application/ld+json"]`)
	count, err := scripts.Count()
	require.NoError(t, err)

	var schema map[string]any
	for i := range count {
		text, err := scripts.Nth(i).TextContent()
		require.NoError(t, err)
		var s map[string]any
		require.NoError(t, json.Unmarshal([]byte(text), &s))
		if s["@type"] == "BreadcrumbList" {
			schema = s
			break
		}
	}
	require.NotNil(t, schema, "BreadcrumbList JSON-LD not found")

	t.Run("has three items", func(t *testing.T) {
		items, ok := schema["itemListElement"].([]any)
		require.True(t, ok, "itemListElement should be an array")
		require.Len(t, items, 3)

		home, _ := items[0].(map[string]any)
		assert.Equal(t, "Home", home["name"])
		assert.Equal(t, float64(1), home["position"])

		section, _ := items[1].(map[string]any)
		assert.Equal(t, "go", section["name"])
		assert.Equal(t, float64(2), section["position"])

		article, _ := items[2].(map[string]any)
		assert.NotEmpty(t, article["name"])
		assert.Equal(t, float64(3), article["position"])
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
