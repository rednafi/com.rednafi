---
title: Periodic readme updates with GitHub Actions
date: 2023-05-04
tags:
    - JavaScript
    - GitHub
---

I recently gave my [blog][blog] a fresh new look and decided it was time to spruce up my
GitHub [profile's][gh-profile] landing page as well. GitHub has a [special][gh-readme]
way of treating the `README.md` file of your <your-username> repo, displaying its
content as the landing page for your profile. My goal was to showcase a brief
introduction about myself and my work, along with a list of the five most recent
articles on my blog. Additionally, I wanted to ensure that the article list stayed up to
date.

There are plenty of fancy GitHub Action workflows like [this][blog-workflow] one that
allows you to add your site's URL to the CI file and it'll periodically fetch the most
recent content from the source and update the readme file. However, I wanted to make a
simpler version of it from scratch which can be extended for periodically updating
any markdown file in any repo, just not the profile readme. So, here's the plan:

* A custom GitHub Action workflow will periodically run a nodejs script.
* The script will then:
    * Grab the XML [sitemap][sitemap] of this blog that you're reading.
    * Parse the XML content and extract the URLs and publication dates of 5 most recent
    articles.
    * Update the associated markdown table with the extracted content on the profile's
    `README.md` file.
* Finally, the workflow will commit the changes and push them to the
[profile][gh-profile-repo] repo. You can see the final outcome [here][gh-profile].

Here's the script that performs the above steps:

```js
// importBlogs.js
/* Import the latest 5 blog posts from rss feed */

import fetch from "node-fetch";
import { Parser } from "xml2js";
import { promises } from "fs";

const rssUrl = "https://rednafi.github.io/index.xml";

const header = `<div align="center">
    Introducing myself...
<div>\n\n`;

const outputFile = "README.md";
const parser = new Parser();

// Define an async function to get and parse the rss data
async function getRssData() {
  try {
    const res = await fetch(rssUrl);
    const data = await res.text();
    return await parser.parseStringPromise(data);
  } catch (err) {
    console.error(err);
  }
}

// Define an async function to write the output file
async function writeOutputFile(output) {
  try {
    await promises.writeFile(outputFile, output);
    console.log(`Saved ${outputFile}`);
  } catch (err) {
    console.error(err);
  }
}

// Call the async functions
getRssData()
  .then((result) => {
    // Get the first five posts from the result object
    const posts = result.rss.channel[0].item.slice(0, 5);

    // Initialize an empty output string
    let output = "";

    // Add a title to the output string
    output += header;

    // Add a header row to the output string
    output += `#### Recent articles\n\n`;
    output += "| Title | Published On |\n";
    output += "| ----- | ------------ |\n";

    // Loop through the posts and add a row for each post to the output string
    for (let post of posts) {
      // Strip the time from the pubDate
      const date = post.pubDate[0].slice(0, 16);
      output += `| [${post.title}](${post.link}) | ${date} |\n`;
    }
    // Call the writeOutputFile function with the output string
    writeOutputFile(output);
  })
  .catch((err) => {
    // Handle the error
    console.error(err);
  });
```

The snippet above utilizes `node-fetch` to make HTTP calls,`xml2js` for XML parsing, and
the built-in `fs` module's `promises` for handling file system operations.

Next, it defines an async function `getRssData` responsible for fetching the XML data
from the [https://rednafi.github.io/index.html][sitemap] URL. It extracts the blog URLs
and publication dates, and returns the parsed data as a list of objects. Another async
function, `writeOutputFile`, writes the parsed XML content as a markdown table and saves
it to the `README.md` file.

The script is executed by the following GitHub Action workflow every day at 0:00 UTC.
Before the CI runs, make sure you create a new [Action Secret][action-secret] named
`ACCESS_TOKEN` that houses an [access token][access-token] with write access to the
repo where the CI runs.

```yaml
# Run a bash script to randomly generate an empty commit to this repo.
name: CI

on:
  # Since we're pushing from this CI, don't run this on the push event because
  # that'll trigger an infinite loop
  # push: [ main ]

  # Add a schedule to run the job every day at 0:00 UTC
  schedule:
    - cron: "0 0 * * *"

  # Allow running this workflow manually
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repo
        uses: actions/checkout@v3
        with:
          # Otherwise, the token used is the GITHUB_TOKEN, instead of your
          # personal access token
          persist-credentials: false
          # Otherwise, there would be errors pushing refs to the destination
          # repository
          fetch-depth: 0

      - uses: actions/setup-node@v3
        with:
          node-version: "lts/*"
          cache: npm
          cache-dependency-path: package-lock.json

      - name: Install dependencies
        run: |
          npm install

      - name: Run linter
        run: |
          npx prettier --write .

      - name: Run script
        run: |
          node scripts/importBlogs.js

      - name: Commit changes
        run: |
          # I want the GitHub bot to make commits on my behalf
          git config --local \
            user.name "github-actions[bot]"
          git config --local \
            user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git add .
          git diff-index --quiet HEAD \
            || git commit -m "Autocommit: updated at $(date -u)"

      - name: Push changes
        uses: ad-m/github-push-action@master
        with:
          github_token: ${{ secrets.ACCESS_TOKEN }}
          branch: ${{ github.ref }}
```

In the first four steps, the workflow checks out the codebase, sets up nodejs, installs
the dependencies, and then runs `prettier` on the scripts. Next, it executes the
`importBlogs.js` script. The script updates the readme and the subsequent shell commands
commit the changes to the repo. The following line ensures that we're only trying to
commit when there's a change in the tracked files.

```sh
git diff-index --quiet HEAD \
  || git commit -m "Autocommit: updated at $(date -u)"
```

Then in the last step, we use an off-the-shelf workflow to push our changes to the
repo. Check out the [workflow][gh-profile-workflow-dir] directory of my profile's repo
to see the whole setup in action. I'm quite satisfied with the final output:

![gh-profile-img]

[blog]: https://rednafi.github.io/
[gh-profile]: https://github.com/rednafi/
[gh-profile-repo]: https://github.com/rednafi/rednafi
[gh-readme]: https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-github-profile/customizing-your-profile/managing-your-profile-readme
[blog-workflow]: https://github.com/gautamkrishnar/blog-post-workflow
[sitemap]: https://rednafi.github.io/index.xml
[action-secret]: https://docs.github.com/en/rest/actions/secrets?apiVersion=2022-11-28
[access-token]: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token
[gh-profile-workflow-dir]: https://github.com/rednafi/rednafi/.github/workflows
[gh-profile-img]: https://user-images.githubusercontent.com/30027932/236357026-0f703a1b-f61a-4af2-ba49-09fe390dea78.png
