---
title: Putting this blog on ATProto with standard.site
date: 2026-06-07
slug: standard-site
atprotoPath: /misc/standard-site/
mermaid: true
tags:
    - Essay
    - DevOps
    - Web
description: >-
  Mirroring a static Hugo blog onto ATProto with standard.site and Sequoia, plus the
  GitHub Actions wiring that republishes the records on every push without any manual
  steps.
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mnpdinpxqp2r"
---

I added [standard.site] support to this blog. Every post now also lives as a record on
[ATProto], the protocol behind Bluesky, and new ones publish themselves whenever I [push to
main].

## What it is

standard.site is two shared [ATProto lexicons], `site.standard.publication` and
`site.standard.document`. The publication record describes the blog: name, URL, icon. Each
post becomes a document record that lives in my own [data repository] on a [PDS] and points
back at the publication. To prove the records are actually mine, there's a
[/.well-known/site.standard.publication] file on my domain and a [link-rel tag] in every
post's HTML pointing at the matching record. Two ends tied together, no central registry in
the middle.

<!-- prettier-ignore-start -->

{{< mermaid >}}
sequenceDiagram
    participant R as Reader
    participant S as rednafi.com
    participant P as PDS

    R->>S: GET /zephyr/carry-the-pager/
    S-->>R: HTML + a site.standard.document link tag
    R->>P: resolve that document record
    P-->>R: site = publication URI, path = /zephyr/carry-the-pager/
    R->>S: GET /.well-known/site.standard.publication
    S-->>R: the same publication URI
    Note over R: URIs match, so it's provably rednafi.com's
{{</ mermaid >}}

<!-- prettier-ignore-end -->

## Why bother

Mostly the previews. A link to one of my posts on Bluesky now shows up as a card with the
title, description, and image instead of a plain URL, because the post is a real record the
network can read. Bluesky [shows richer previews] for standard.site links now. Beyond that,
the records live in my own PDS, so any indexer or reader can pick them up, and they turn up
in readers like [docs.surf] on their own. And it's cheap [POSSE]: rednafi.com stays the
canonical copy, a copy syndicates out to the [ATmosphere].

## Setting it up with Sequoia

I didn't hand-roll any of the ATProto plumbing. [Sequoia] is a CLI by Steve Simkins that
does the whole thing for static sites, and it already speaks Hugo, Astro, Jekyll, and the
rest. If you want to put your own blog on standard.site, it goes roughly like this.

First, get an ATProto identity, since the records live in your own PDS. A Bluesky account is
one. Ownership is checked against a domain, so it helps to set your site's domain as your
handle (mine is `rednafi.com`) and mint an app password for the CLI to log in with.

Then run `sequoia init` in the repo. It authenticates against your PDS, creates a
`site.standard.publication` record describing the blog (name, URL, icon), and scaffolds a
`sequoia.json`. That config is small: it points at your content directory and maps the two
frontmatter fields it cares about, the publish date and the slug.

```json
{
  "siteUrl": "https://rednafi.com",
  "contentDir": "content",
  "publicationUri": "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.publication/3mnl6f7ob462z",
  "frontmatter": { "publishDate": "date", "slugField": "slug" }
}
```

That `publicationUri` is the `at://` address of the publication record `init` just made,
which is where it comes from. The same URI also lands in
`static/.well-known/site.standard.publication`, so the domain and the record name each other
and the ownership check holds.

Each post's HTML also needs a `<link rel="site.standard.document">` pointing at that post's
record. `sequoia inject` can patch the tags into your built HTML; I emit them from my Hugo
`head` partial instead.

With that wired up, `sequoia publish` walks the content, creates a `site.standard.document`
record per post, and writes the resulting `atUri` back into each post's frontmatter. State
lives in `.sequoia-state.json`, so reruns only touch what actually changed.

## Making it hands-free

I didn't want to run `sequoia publish` by hand, so it happens in [CI]. Two pieces make that
work.

A [small Go script] fills in one frontmatter field before Sequoia runs. standard.site wants
a stable `atprotoPath` per document, and rather than type that into every post I derive it
from the file's location and slug, so `content/zephyr/carry_the_pager.md` with
`slug: carry-the-pager` gets `atprotoPath: /zephyr/carry-the-pager/` written in. It also
fails the build if a post is missing one.

Then GitHub Actions handles the rest on every push to `main`: run that sync script,
`sequoia publish` with my handle and app password from repo secrets, prettier-format the
metadata Sequoia generated, then commit the new `atUri`s, the `.sequoia-state.json`, and the
`.well-known` file back with a `[skip ci]` tag before Hugo builds and deploys.

```yaml
- name: Sync standard.site frontmatter
  run: go run ./scripts/stdsitesync

- name: Publish standard.site records
  env:
    ATP_IDENTIFIER: ${{ secrets.ATP_IDENTIFIER }}
    ATP_APP_PASSWORD: ${{ secrets.ATP_APP_PASSWORD }}
  run: npx -y sequoia-cli publish
```

So my actual routine didn't change at all. Write Markdown, push to `main`, walk away. The
ATProto side catches up by itself. This very post turned into a `site.standard.document` the
moment the deploy ran.

## Seeing it work

Here's the part I actually wanted. I share a post on Bluesky and it unfurls into a card
built from the record:

![Bluesky rendering a rednafi.com post as a rich preview card][img_1]

And the same post sitting on the network as its `site.standard.document` record, viewed
through [pdsls]. Same title and description the card used, plus the path, tags, and the full
body, all as portable data instead of only HTML:

![The same post as a site.standard.document record in pdsls][img_2]

The [config], the [script], and the [ci workflow] are all in the repo if you want to grab
the setup.

<!-- references -->
<!-- prettier-ignore-start -->

[standard.site]:
    https://standard.site

[atproto]:
    https://atproto.com

[atproto lexicons]:
    https://atproto.com/specs/lexicon

[data repository]:
    https://atproto.com/guides/data-repos

[pds]:
    https://atproto.com/guides/glossary#pds-personal-data-server

[/.well-known/site.standard.publication]:
    https://rednafi.com/.well-known/site.standard.publication

[link-rel tag]:
    https://github.com/rednafi/rednafi.com/blob/main/layouts/partials/head.html

[shows richer previews]:
    https://atproto.com/blog/standard-site-bluesky-timeline

[posse]:
    https://indieweb.org/POSSE

[atmosphere]:
    https://atproto.com/blog/indexing-standard-site

[sequoia]:
    https://sequoia.pub

[ci]:
    https://github.com/rednafi/rednafi.com/blob/main/.github/workflows/ci.yml#L52-L78

[docs.surf]:
    https://docs.surf

[pdsls]:
    https://pdsls.dev/at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.publication/3mnl6f7ob462z

[push to main]:
    https://github.com/rednafi/rednafi.com

[config]:
    https://github.com/rednafi/rednafi.com/blob/main/sequoia.json

[script]:
    https://github.com/rednafi/rednafi.com/blob/main/scripts/stdsitesync/main.go

[ci workflow]:
    https://github.com/rednafi/rednafi.com/blob/main/.github/workflows/ci.yml

[small Go script]:
    https://github.com/rednafi/rednafi.com/blob/main/scripts/stdsitesync/main.go

[img_1]:
    https://blob.rednafi.com/static/images/standard_site/img_1_v2.png

[img_2]:
    https://blob.rednafi.com/static/images/standard_site/img_2.png

<!-- prettier-ignore-end -->
