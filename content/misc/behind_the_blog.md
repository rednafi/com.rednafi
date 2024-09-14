---
title: Behind the blog
date: 2024-09-14
tags:
  - Essays
---

When I started writing here about five years ago, I made a promise to myself that I wouldn't
give in to the trend of starting a blog, adding one overly enthusiastic entry about the
stack behind it, and then vanishing into the ether.

I was somewhat successful at that and wanted to write something I can link to when people
are curious about the machinery that drives this site. The good thing is that the technology
stack is simple and hasn't changed much over the years as I've refrained from tinkering with
it unless absolutely necessary.

## Markdown

I write plain Markdown files in my editor of choice, which has been VSCode since its launch.
Once I'm finished, pre-commit[^1] runs a fleet of linters like Prettier[^2] and
Blacken-docs[^3] to fix line length and code formatting.

## Hugo

Hugo[^4] is the static site generator that converts the Markdown files into HTML. I went
with it because I needed something that generates the site almost instantly, even with a ton
of content. I don't get to write Go professionally, and messing around with Hugo templates
or its source code is a good excuse to play with some Go.

Initially, I tried some JS-based SSGs and abandoned them quickly because I found myself
struggling to keep up with the pace at which the community likes to reinvent tooling. I use
the Papermod[^5] theme and tweaked the CSS here and there over the years. Papermod takes
care of all the pesky SEO stuff that I like to tell myself I don't care about at all.

## GitHub Issues

I use GitHub Issues[^6] to brainstorm ideas for a new piece and record my writings. Usually,
I go about collecting ideas over the week, record them in Issues, and write something over
the weekend. This workflow was heavily inspired by Simon Willison's text on his work
process[^7].

![github issues as a research notebook][image_1]

## GitHub Actions and GitHub Pages

Once I push the content to the mainline, GitHub Actions[^8] kicks in, checks linter
conformity, builds the site, and pushes it to GitHub Pages[^9]. There's nothing to maintain,
and I don't need to worry about scaling when some of my writing appears on the front page of
Hacker News. Apart from the domain, maintaining this site costs me nothing, and I intend to
maintain the status quo.

## Cloudflare Cache and R2

I'm a huge fan of Cloudflare and often try to shoehorn their offerings into my projects.
Since my domain is registered with them, it took me a minute to set up their proxy with my
domain's DNS and turn on caching. Their caching layer absorbs the majority of the traffic,
and on average, less than 10% of the requests reach the origin server. One added bonus of
having the proxy layer is that I have access to more accurate analytics.

![cloudflare cache analytics][image_2]

Static assets like images, CSS, JS, and other artifacts live on Cloudflare R2[^10]. I used
to use GitHub Issues to host my images and served the CSS and JS from the origin. I've
recently changed that to use R2, which allows me to manage everything from a single place
without worrying about cost. Their free version is incredibly generous as there is no egress
bandwidth fee, and because of the caching layer, I don't use much of the allocated quota.
It's fantastic!

![cloudflare r2][image_3]

## Oxipng

Oxipng[^11] is used to compress the images before uploading them to Cloudflare R2 bucket
with the Wrangler[^12] CLI. The Makefile in the repo has a single command named
`upload-static`, that does all of this in a single pass.

```make
upload-static:
    oxipng -o 6 -r static/images/
    find static -type f | while read filepath; do \
        key=$$(echo "$$filepath" | sed 's|^|blog/|'); \
        wrangler r2 object put $$key --file "$$filepath"; \
    done
```

I just put the screenshots and images in the `/static/images/<blog-name>/*.png` path, update
the references in the relevant Markdown file, and run `make upload-static` before pushing
the changes to the repo.

## Google Analytics

I'm still using Google Analytics[^13], which I don't like much. Cloudflare already gives me
more accurate insight into the traffic, but the free version doesn't allow me to see which
page is getting how many hits. I might bite the bullet in the future and just pay Cloudflare
to be able to remove intrusive and heavy analytics scripts from the site.

Everything is publicly available[^14] on GitHub.

[^1]: [Pre-commit](https://pre-commit.com/)

[^2]: [Prettier](https://prettier.io/)

[^3]: [Blacken-docs](https://pypi.org/project/blacken-docs/)

[^4]: [Hugo](https://gohugo.io/)

[^5]: [Hugo Papermod](https://github.com/adityatelange/hugo-PaperMod)

[^6]:
    [I usually use GitHub Issues like this](https://github.com/rednafi/rednafi.com/issues/125)

[^7]:
    [Coping strategies for the serial project hoarder -- Simon Willison's](https://simonwillison.net/2022/Nov/26/productivity/)

[^8]: [GitHub Actions](https://github.com/features/actions)

[^9]: [GitHub Pages](https://pages.github.com/)

[^10]: [Cloudflare R2](https://developers.cloudflare.com/r2/)

[^11]: [Oxipng](https://github.com/shssoichiro/oxipng)

[^12]: [Cloudflare Wrangler](https://developers.cloudflare.com/workers/wrangler/)

[^13]: [Google Analytics](https://analytics.google.com/)

[^14]: [GitHub - rednafi.com](https://github.com/rednafi/rednafi.com)

[image_1]: https://blob.rednafi.com/static/images/behind_the_blog/img_1.png
[image_2]: https://blob.rednafi.com/static/images/behind_the_blog/img_2.png
[image_3]: https://blob.rednafi.com/static/images/behind_the_blog/img_3.png
