---
title: Behind the blog
date: 2024-09-14
tags:
  - Essay
---

When I started writing here about five years ago, I made a promise to myself that I wouldn't
give in to the trend of starting a blog, adding one overly enthusiastic entry about the
stack behind it, and then vanishing into the ether.

I was somewhat successful at that and wanted to write something I can link to when people
are curious about the machinery that drives this site. The good thing is that the tech stack
is simple and has remained stable over the years since I've only made changes when
absolutely necessary.

## Markdown

I write plain Markdown files in my editor of choice, which has been VSCode since its launch.
Once I'm finished, [pre-commit] runs a fleet of linters like [prettier] and [blacken-docs]
to fix line length and code formatting.

## Hugo

[Hugo] is the static site generator that turns the Markdown files into HTML. I chose it
because I needed something that can build the site quickly, even with lots of content. It
lets me hot reload the server and check my changes as I write. Plus, I don't get to write Go
at work, so messing with Hugo templates or its source code gives me a reason to play around
with Go.

I initially tried some JS-based SSGs but dropped them pretty quickly because I couldn't keep
up with the constant tooling changes in the JavaScript universe. I use the [papermod] theme
and have tweaked the CSS over time. Papermod handles the SEO stuff, which I like to pretend
I don't care about.

## GitHub Issues

I use [GitHub Issues] to brainstorm ideas and keep track of my writing. I usually gather
ideas throughout the week, log them in Issues, and then write something over the weekend.
This workflow is heavily inspired by Simon Willison's piece on his work [process].

![image_1]

## GitHub Actions and GitHub Pages

Once I push content to the main branch, [GitHub Actions] automatically runs, checks the
linter, builds the site, and deploys it to [GitHub Pages]. There's nothing to maintain, and
I don't have to worry about scaling, even if one of my posts hits the front page of Hacker
News. Aside from the domain, this site costs me nothing to run, and I plan to keep it that
way.

## Cloudflare Cache and R2

I'm a huge fan of Cloudflare and often try to shoehorn their offerings into my projects.
Since my domain is registered with them, setting up their proxy with my domain's DNS and
turning on caching took just a few minutes. Their caching layer absorbs most of the traffic,
and less than 10% of the requests hit the origin server. Plus, having the proxy layer gives
me access to more accurate analytics.

![image_2]

Static assets like images, CSS, JS, and other files are stored on [Cloudflare R2]. I used to
host my images with GitHub Issues and serve CSS and JS from the origin, but I recently
switched everything to R2. Now I can manage it all from one place without worrying about
costs. Their free plan is super generous—there's no egress bandwidth fee, and because of
caching, I barely use any of the quota. It's fantastic!

![image_3]

## Oxipng

[Oxipng] is used to compress images before uploading them to the Cloudflare R2 bucket with
the [wrangler] CLI. The Makefile in the repo has a single command called `upload-static`
that handles everything in one go.

```make
upload-static:
    oxipng -o 6 -r static/images/
    find static -type f | while read filepath; do \
        key=$$(echo "$$filepath" | sed 's|^|blog/|'); \
        wrangler r2 object put $$key --file "$$filepath"; \
    done
```

I just drop the screenshots and images into `/static/images/<blog-name>/*.png`, update the
references in the Markdown file, and run `make upload-static` before pushing the changes to
the repo.

## Google Analytics

I'm still using [Google Analytics], even though I'm not a huge fan. Cloudflare already
gives me better traffic insights, but the free version doesn't show how many hits each page
gets. At some point, I might just pay for Cloudflare's upgraded plan so I can get rid of the
bulky, intrusive analytics scripts for good.

The [source code] and content for this site are all publicly available on GitHub.

<!-- Resources -->
<!-- prettier-ignore-start -->

[pre-commit]:
    https://pre-commit.com/

[prettier]:
    https://prettier.io/

[blacken-docs]:
    https://pypi.org/project/blacken-docs/

[hugo]:
    https://gohugo.io/

[papermod]:
    https://github.com/adityatelange/hugo-PaperMod

<!-- i usually use github issues like this -->
[github issues]:
    https://github.com/rednafi/rednafi.com/issues/125

<!-- coping strategies for the serial project hoarder -- simon willison -->
[process]:
    https://simonwillison.net/2022/Nov/26/productivity/

[github actions]:
    https://github.com/features/actions

[github pages]:
    https://pages.github.com/

[cloudflare r2]:
    https://developers.cloudflare.com/r2/

[oxipng]:
    https://github.com/shssoichiro/oxipng

<!-- cloudflare wrangler -->
[wrangler]:
    https://developers.cloudflare.com/workers/wrangler/

[google analytics]:
    https://analytics.google.com/

<!-- source code of this site -->
[source code]:
    https://github.com/rednafi/rednafi.com

<!-- github issues as a research notebook -->
[image_1]:
    https://blob.rednafi.com/static/images/behind_the_blog/img_1.png

<!-- cloudflare cache analytics -->
[image_2]:
    https://blob.rednafi.com/static/images/behind_the_blog/img_2.png

<!-- cloudflare r2 dashboard -->
[image_3]:
    https://blob.rednafi.com/static/images/behind_the_blog/img_3.png

<!-- prettier-ignore-end -->
