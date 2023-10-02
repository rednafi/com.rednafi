# [Redowan's Reflections][site]

[![pre-commit.ci status][precommit-svg]][this]

Musings & rants on software. Find them at [rednafi.com][site].

## Local development

-   Install [Hugo][hugo]. I'm on macOS and Hugo can be installed with `brew`:

    ```sh
    brew install hugo
    ```

-   Bootstrap the theme:

    ```sh
    make init
    ```

-   Update the theme:

    ```sh
    make update
    ```

-   Run the local server:

    ```sh
    make devserver
    ```

-   Go to [http://localhost:1313][localhost] to access the site locally.

## Deployment

The site is deployed to GitHub Pages via GitHub Actions.

[site]: https://rednafi.com
[hugo]: https://gohugo.io/
[localhost]: http://localhost:1313
[precommit-svg]: https://results.pre-commit.ci/badge/github/rednafi/rednafi.com/main.svg
[this]: https://results.pre-commit.ci/latest/github/rednafi/rednafi.com/main
