# Redowan's Reflections

[![pre-commit.ci status][precommit-svg]][precommit-status]

Musings & rants on software. Find them at [rednafi.com].

## Local development

- Install [Hugo]. I'm on macOS and Hugo can be installed with `brew`:

    ```sh
    brew install hugo
    ```

- Bootstrap:

    ```sh
    make init
    ```

- Update the stack:

    ```sh
    make update
    ```

- Run the local server:

    ```sh
    make dev
    ```

- Go to [http://localhost:1313] to access the site locally.

## Deployment

The site is deployed to GitHub Pages via GitHub Actions.

[rednafi.com]: https://rednafi.com
[hugo]: https://gohugo.io/
[http://localhost:1313]: http://localhost:1313
[precommit-svg]: https://results.pre-commit.ci/badge/github/rednafi/rednafi.com/main.svg
[precommit-status]: https://results.pre-commit.ci/latest/github/rednafi/rednafi.com/main
