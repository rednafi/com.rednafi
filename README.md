<div align="center">

# [Reflections][site]

</div>

Musings & rants on software. Find them at [rednafi.github.io][site].

## Local development

* Install [Hugo][hugo]. I'm on macOS and Hugo can be installed with `brew`:
    ```
    brew install hugo
    ```
* Bootstrap the theme:
    ```
    make init
    ```
* Update the theme:
    ```
    brew update
    ```
* Run the local server:
    ```
    make devserver
    ```
* Go to [http://localhost:1313][localhost] to access the site locally.

## Deployment

The site is deployed to GitHub Pages via GitHub Actions.


[site]: https://rednafi.github.io
[hugo]: https://gohugo.io/
[localhost]: http://localhost:1313
