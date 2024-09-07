---
title: Automerge Dependabot PRs on GitHub
date: 2022-07-07
tags:
    - GitHub
---

Whether I'm trying out a new tool or just prototyping with a familiar stack, I usually
create a new project on GitHub and run all the experiments there. Some examples of these
are:

-   [rubric]: linter config initializer for Python
-   [exert]: declaratively apply converter functions to class attributes
-   [hook-slinger]: generic service to send, retry, and manage webhooks
-   [think-async]: exploring cooperative concurrency primitives in Python
-   [epilog]: container log aggregation with Elasticsearch, Kibana & Filebeat

While many of these prototypes become full-fledged projects, most end up being just one-time
journies. One common theme among all of these endeavors is that I always include
instructions in the `readme.md` on how to get the project up and running—no matter how small
it is. Also, I tend to configure a rudimentary CI pipeline that runs the linters and tests.
GitHub Actions and Dependabot[^1] make it simple to configure a basic CI workflow.
Dependabot keeps the dependencies fresh and makes pull requests automatically when there's a
new version of a dependency used in a project.

Things can get quickly out of hand if you've got a large collection of repos where the
automated CI runs periodically. Every now and then, I get a sizable volume of PRs in these
fairly stale repos that I still want to keep updated. Merging these manually is a chore.
Luckily, there are multiple ways[^2] that GitHub offers to automatically merge PRs. The
workflow that is documented here is the one I happen to like the most. I also think that
this process leads to the path of the least surprise. Instead of depending on a bunch of
GitHub settings, we'll write a GitHub action workflow[^3] to automate the process.

First, you'll need to turn on the auto-merge option from the repository settings. To do so,
go to the repo's _settings_ tab and turn on the _Allow auto-merge_ option from the _Pull
Requests_ section:

![turn on automerge][image_1]

Now, you probably don't want to mindlessly merge every pull request Dependabot throws at
you. You most likely want to make sure that a pull request triggers certain tests and it'll
be merged only if all of those checks pass. To do so, you can turn on branch protection[^4].
From the _settings_ panel, select _Branches_ on the left panel:

![select up a branch protection rule][image_2]

Once you've selected the tab, add a branch protection rule to the target branch against
which Dependabot will send the pull requests:

![set up a branch protection rule][image_3]

In this case, I'm adding the protection layer to the `main` branch. I've turned on the
_Require status checks to pass before merging_ toggle and added the `build` step to the list
of status checks that are required. Here, you can select any job from your CI files in the
`.github/workflows` directory:

![turn on status check][image_4]

Once this is done, you can drop the following CI file in the `.github/workflows` directory
of your repo. It's the same file[^5] that's currently living inside this site's CI folder.

```yml
# .github/workflows/automerge.yml

name: Dependabot auto-merge

on: pull_request

permissions:
  contents: write
  pull-requests: write  # Needed if in a private repository

jobs:
  dependabot:
    runs-on: ubuntu-latest
    if: ${{ github.actor == 'dependabot[bot]' }}
    steps:
      - name: Enable auto-merge for Dependabot PRs
        run: gh pr merge --auto --merge "$PR_URL"
        env:
          PR_URL: ${{github.event.pull_request.html_url}}
          # GitHub provides this variable in the CI env. You don't
          # need to add anything to the secrets vault.
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

From now on, every time Dependabot sends a merge request, the checks will be triggered and
if all the mandatory checks pass, the `automerge.yml` workflow will merge it into the target
branch.

[^1]:
    [Configuring Dependabot security updates](https://docs.github.com/en/code-security/dependabot/dependabot-security-updates/configuring-dependabot-security-updates)

[^2]:
    [Automatically merging a pull request](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/incorporating-changes-from-a-pull-request/automatically-merging-a-pull-request)

[^3]:
    [Enable auto-merge on a pull request](https://docs.github.com/en/code-security/dependabot/working-with-dependabot/automating-dependabot-with-github-actions#enable-auto-merge-on-a-pull-request)

[^4]:
    [About protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)

[^5]:
    [Complete CI file](https://github.com/rednafi/reflections/blob/master/.github/workflows/automerge.yml)

[rubric]: https://github.com/rednafi/rubric
[exert]: https://github.com/rednafi/exert
[hook-slinger]: https://github.com/rednafi/hook-slinger
[think-async]: https://github.com/rednafi/think-async
[epilog]: https://github.com/rednafi/epilog
[image_1]:
    https://blob.rednafi.com/static/images/automerge_dependabot_prs_on_github/img_1.png
[image_2]:
    https://blob.rednafi.com/static/images/automerge_dependabot_prs_on_github/img_2.png
[image_3]:
    https://blob.rednafi.com/static/images/automerge_dependabot_prs_on_github/img_3.png
[image_4]:
    https://blob.rednafi.com/static/images/automerge_dependabot_prs_on_github/img_4.png
