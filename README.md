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

The deployment workflow also publishes Standard.site records through the standard Sequoia
workflow before the Hugo build. GitHub Actions needs these repository secrets:

- `ATP_IDENTIFIER`: the ATProto handle for the site, currently `rednafi.com`
- `ATP_APP_PASSWORD`: an ATProto app password with repo write access

CI derives `atprotoPath` for the site's multi-section Hugo URLs, runs `sequoia publish`,
formats generated metadata, and commits generated `atUri`/Sequoia state changes back with
`[skip ci]`.

[rednafi.com]: https://rednafi.com
[hugo]: https://gohugo.io/
[http://localhost:1313]: http://localhost:1313
[precommit-svg]: https://results.pre-commit.ci/badge/github/rednafi/rednafi.com/main.svg
[precommit-status]: https://results.pre-commit.ci/latest/github/rednafi/rednafi.com/main
