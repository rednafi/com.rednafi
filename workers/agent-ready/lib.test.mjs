import assert from "node:assert/strict";
import test from "node:test";

import {
  appendHeaderToken,
  buildLinkHeaderValues,
  buildMarkdownResponse,
  htmlToAgentMarkdown,
  prefersMarkdown,
  withDiscoveryHeaders,
} from "./lib.mjs";

test("prefers markdown only when it is competitive in Accept negotiation", () => {
  assert.equal(prefersMarkdown("text/markdown"), true);
  assert.equal(prefersMarkdown("text/html, text/markdown;q=0.4"), false);
  assert.equal(prefersMarkdown("text/markdown, text/html;q=0.5"), true);
  assert.equal(prefersMarkdown("*/*"), false);
});

test("appendHeaderToken avoids duplicate Vary entries", () => {
  assert.equal(appendHeaderToken("", "Accept"), "Accept");
  assert.equal(appendHeaderToken("Accept-Encoding", "Accept"), "Accept-Encoding, Accept");
  assert.equal(appendHeaderToken("Accept, Accept-Encoding", "Accept"), "Accept, Accept-Encoding");
});

test("buildLinkHeaderValues advertises markdown and llms discovery", () => {
  const links = buildLinkHeaderValues(new URL("https://rednafi.com/go/anemic-stack-traces/"));
  assert.equal(links.length, 3);
  assert.match(links[0], /rel="alternate"; type="text\/markdown"/);
  assert.match(links[1], /application\/rss\+xml/);
  assert.match(links[2], /\/llms\.txt/);
});

test("withDiscoveryHeaders appends Link headers without replacing the response body", async () => {
  const response = new Response("<html></html>", {
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      Vary: "Accept-Encoding",
    },
  });

  const out = withDiscoveryHeaders(response, new URL("https://rednafi.com/"));
  assert.equal(await out.text(), "<html></html>");
  assert.equal(out.headers.get("Vary"), "Accept-Encoding, Accept");
  assert.match(out.headers.get("Link"), /text\/markdown/);
  assert.match(out.headers.get("Link"), /llms\.txt/);
});

test("htmlToAgentMarkdown produces readable markdown with absolute links", () => {
  const html = `<!doctype html>
    <html lang="en">
      <head>
        <title>Sample Post | Redowan's Reflections</title>
        <meta name="description" content="A short summary for agents.">
      </head>
      <body>
        <header><a href="/about/">About</a></header>
        <main id="main">
          <article>
            <h1>Sample Post</h1>
            <p>Hello <a href="/about/">there</a>.</p>
            <ul><li>First point</li><li>Second point</li></ul>
          </article>
        </main>
      </body>
    </html>`;

  const markdown = htmlToAgentMarkdown(html, new URL("https://rednafi.com/go/sample-post/"));
  assert.match(markdown, /^# Sample Post \| Redowan's Reflections/m);
  assert.match(markdown, /Source: https:\/\/rednafi\.com\/go\/sample-post\//);
  assert.match(markdown, /A short summary for agents\./);
  assert.match(markdown, /\[there\]\(https:\/\/rednafi\.com\/about\/\)/);
  assert.match(markdown, /\* First point/);
});

test("buildMarkdownResponse returns the markdown variant with correct headers", () => {
  const origin = new Response("<html></html>", {
    status: 200,
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      ETag: "\"abc123\"",
    },
  });

  const out = buildMarkdownResponse(
    origin,
    new URL("https://rednafi.com/"),
    "# Home\n\nSource: https://rednafi.com/\n",
  );

  assert.equal(out.headers.get("Content-Type"), "text/markdown; charset=utf-8");
  assert.equal(out.headers.get("ETag"), null);
  assert.equal(out.headers.get("Vary"), "Accept");
  assert.match(out.headers.get("Link"), /text\/markdown/);
});
