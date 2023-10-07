---
title: Footnotes for the win
date: 2023-10-07
tags:
    - Meta
---

There are a few ways you can add URLs to your markdown documents:

-   Inline links

    ```md
    [inline link](https://example.com)
    ```

    This will render as [inline link](https://example.com).

-   Reference links

    ```md
    [reference link]
    ```

    Define the link destination elsewhere in the document like this:

    ```md
    [reference link]: https://example.com
    ```

    This will render the same way as before, [reference link].

-   Footnote style reference links

    ```md
    footnote style reference link[^1]
    ```

    Define the link destination using a footnote reference:

    ```md
    [^1]: https://example.com
    ```

    This will render a bit differently with a clickable number beside the origin text that
    refers to the backref at the bottom of the document. Like this: footnote style reference
    link[^1].

    Try clicking on the number within the square brackets and see how the page scrolls down
    to the corresponding backref link that lives with other backrefs at the tail of the
    page.

The inline link approach is the most prevalent one and is also the easiest one to write. But
it suffers from a few issues:

-   Links scattered throughout your documents can make updates cumbersome.
-   Reusing a link elsewhere will require multiple copy-pastes.
-   Placing several [links][reference link] [side][reference link] by [side][reference link]
    can feel awkward, and URL styling like the blue highlighting or underlining makes things
    noisy.
-   To add a reference section, you'll have to create a separate segment at the bottom of
    your page and duplicate the URLs.
-   On mobile devices, accidentally tapping a URL can promptly redirect readers away from
    your content, potentially against their intention.
-   Enforcing a line width limit can be challenging due to lengthy inlined URLs.

The reference link approach solves some of these issues since you won't have to scatter the
URLs across your document or repeat them multiple times for multiple usage. This also allows
you to use a markdown formatter to enforce maximum line width. I use prettier[^2] to cap the
line width at 92 characters and the formatter works better when it doesn't have to shimmy
around multiple long inline URLs.

This is certainly better than using inline links, but it still suffers from all the other
issues that plague the former approach. Creating a reference section still requires some
repetition, and juxtaposing multiple links remains awkward. Also, accidental misclicks that
take you to a different page remains an issue.

The footnote-style reference link comes to the rescue. It keeps the document clean by moving
all URLs to the bottom in a dedicated reference section. The small superscript numbers don't
distract the reader as much but provide an easy way to navigate to the corresponding links
if needed. Accidental clicks are no longer an issue since clicking on a reference
superscript will bring the user down to the footnote section where they can click on the
concomitant URL or jump back to the origin by tapping on the backref (↩︎) symbol. The
reference section also allows you to provide more context on each link, like a title or
description.

Moreover, adding multiple links to the same target is straightforward since you simply add
the footnote numbers like this[^3][^4]. Plus, you don't have to manually create a separate
reference section; it automatically gets created for you as you start adding footnotes. See
the reference section in this post and click on the backref links to go back to the origin.
Most parsers like GitHub flavored markdown[^5] now support footnotes out of the box.

Recently, I've spent an entire evening converting almost all of the inline links on this
site into footnote style references in a semi-automated manner. I still use reference links
here and there but mostly prefer footnotes since they allow me to avoid repetition and
subjectively look less distracting compared to underlined or highlighted URLs. And suddenly,
prettier's[^2] job has become easier too!

[^1]: https://example.com
[^2]: https://prettier.io/
[^3]: Footnotes with extra texts <https://rednafi.com>
[^4]:
    Multiple footnotes are less distracting than multiple side-by-side URLs
    <https://rednafi.com/index>

[^5]: [GitHub flavored markdown](https://github.github.com/gfm/)
[^6]:
    [Checkout the raw markdown file of this post](https://github.com/rednafi/rednafi.com/blob/main/content/zephyr/footnotes_for_the_win.md)

[reference link]: https://example.com
