---
title: Auditing commit messages on GitHub
date: 2022-10-06
tags:
    - GitHub
---

After reading Simon Willison's amazing [piece][1] on how he adds new features to his
open-source softwares, I wanted to adopt some of the good practices and incorporate them
into my own workflow. One of the highlights of that post was how to kick off a feature
work. The process roughly goes like this:

* Opening a new GitHub issue for the feature in the corresponding repository.
* Adding a rough description of the feature to the issue.
* Creating a feature branch off of `main/master/trunk`. If the feature is trivial or
just a doc update, this step can be skipped.
* Referring to the issue in every commit message as you start working on the feature:

    * Appending `#refs <issue-number>` to every commit message. This will attach the
    commit to the concerning issue on the GitHub UI.
    * Appending `#closes <issue-number>` to the final commit message when the feature
    is complete.
    * If you need to refer to an issue after it's closed, you can still do that by
    appending `#refs <issue-number>` to the commit message.

    So a commit message should look similar to `Feature foo, refs #120` or
    `Update foo, closes #115`

    The comma (`,`) before `refs/closes` is essential here. I like to enforce it.

This pattern can also work for bugfixes without any changes. Here's an [example][2] of
it in action. I follow the pattern to write the blogs on this site as well. This is
what a feature issue might look like on GitHub:

![image](https://user-images.githubusercontent.com/30027932/194779762-2000b766-3efa-421c-be77-757233e1e8f2.png)

While I'm quite happy with how the process is working for me, often time, I get
careless and push commits without a reference to any issue. This pollutes the Git
history and breaks my streak of maintaining good hygiene. So, I was looking for a way
to make sure that the CI fails and reprimands me whenever I'm not following the process
correctly. It's just one less thing to worry about.

I've decided to use GitHub Actions to audit the conformity of the commit messages. The
CI pipeline is orchestrated as follows:

* After every push and pull-request, the `audit-commits` job in an `audit.yml` workflow
file will verify the conformity of the commit messages. This job runs a regex pattern
against every commit message and fails with exit code 1 if the message doesn't respect
the expected format.
* If the `audit-commits` job passes successfully, only then the primary jobs in the
`ci.yml` workflow will execute. The entire pipeline will fail and the primary CI
workflow won't be triggered at all if the `audit-commit` job fails at any point.

On GitHub, you're expected to place your workflow files in the `.github/workflows`
directory. If you inspect this blog's [workflows][3] folder, you'll see this pattern in
action. Here the directory has three workflow files:

```
.github/workflows
├── audit.yml
├── automerge.yml
└── ci.yml
```

The `automerge.yml` file automatically merges a pull-request when the primary CI jobs
pass. I wrote about it in more detail in [another][4] write-up. We'll ignore the
`automerge.yml` file for now. Here, the audit file runs after every push and
pull-request and verifies the structure of the commit message. I picked a generic name
like `audit.yml` instead of a more specific one like `audit-commit.yml` because in the
future if I want to add another check, I can easily extend this file without renaming
it. Here's the unabridged content of the `audit.yml` file:

```yaml
# .github/workflows/audit.yml
# Auditing commit structure.
name: Audit

on:
  workflow_call:


jobs:
  audit-commits:
    runs-on: ubuntu-latest
    if: ${{ github.actor != 'dependabot[bot]' }}
    steps:
      - name: "Return exit code 1 if the commit messages aren't formatted correctly."
        shell: bash
        run: |
          set -euo pipefail

          # Get the commit payload from GH Actions event.
          # https://docs.github.com/en/developers/webhooks-and-events/events/
          # github-event-types#pushevent
          commits='${{ toJSON(github.event.commits) }}'

          # Exit with 0 if no new commit is found.
          if [[ $commits =~ "null" ]]; then
              echo "No commit found. Exiting..."
          exit 0
          fi

          # Get the unique messages from the commits event.
          parsed=$(echo -n "$commits" | jq -r ".[].message" | sort -u)
          mtch='(, refs|, closes) #[0-9]+'

          echo "$parsed" | while IFS= read -r raw_line; do
              line=$(echo "$raw_line" | tr -d "\r\n")

              # Ignore empty lines.
              if [[ -z "$line" ]]; then
                  continue

              # Check with regex if the commit message contains 'refs #issue_number'
              # or 'closes #issue_number'. If not, exit with an error.
              elif [[ "$line" =~ $mtch ]]; then
                  echo "Commit message: $line ✅"
              else
                  echo "Commit message: $line ❌"
                  echo -n "Commit message must contain "
                  echo -n "'refs #issue_number' or 'closes #issue_number'."
                  exit 1
              fi
          done
```

I've defined this workflow as a reusable one. A reusable workflow can be called like a
function with parameters from another workflow. The `workflow_call` node the `audit.yml`
file makes it a reusable one and you can define additional parameters in this section if
you need to do so. However, in this particular case, I don't need to pass any parameters
while calling the `audit.yml` workflow from the `ci.yml` workflow. You can find more
details on how to define reusable workflows in the docs [here][5].

In the `jobs` section of the `audit.yml` file, we define a single `audit-commits` job
that runs a bash script against every incoming commit message and verifies its
structure. The commit messages can be accessed from the
`'${{ toJSON(github.event.commits) }}'` context variable. Then the script loops over
every commit message and verifies the structure. It'll terminate the job with exit code
`1` if the incoming message doesn't match the expected structure. Otherwise, the script
will gracefully terminate the job with exit code `0`.

In the main `ci.yml` file the `audit.yml` workflow is called like this:

```yaml
...

jobs:
  audit:
    uses: rednafi/reflections/.github/workflows/audit.yml@master
...
```

The `ci.yml` file roughly looks like this:

```yaml
name: CI

on:
  push:
  pull_request:

  # Everyday at 0:37 UTC.
  schedule:
    - cron: "37 0 * * *"


# Cancel any running workflow if the CI gets triggered again.
concurrency:
  group: ${{ github.head_ref || github.run_id }}
  cancel-in-progress: true


jobs:
  audit:
    uses: rednafi/reflections/.github/workflows/audit.yml@master

  build:
    needs: ["audit"]
    runs-on: ubuntu-latest
    steps:
      ...

  test:
    needs: ["build"]
    runs-on: ubuntu-latest
    steps:
      ...

  deploy:
    needs: ["deploy"]
    runs-on: ubuntu-latest
    steps:
      ...
```

Here the `needs: ["audit"]` node in the `build` section ensures that the build will only
trigger if the `audit` job passes successfully. Otherwise, none of the `build`,
`test`, or `deploy` jobs will run and the CI will fail with a non-zero exit code. You
can see the fully working `ci.yml` file [here][6].

## Notes

GitHub Actions terminology can be confusing.

* A **workflow** is a separate file that contains one or more **jobs**.
* A **job** is a set of steps in a workflow that executes on the same **runner**.
* A **runner** is a server that runs your workflows when they're triggered. Each runner
can run a single job at a time.
* A **reusable** workflow can be called from another workflow file.

You can learn more about the terminologies [here][7].

## References

[1]: https://simonwillison.net/2022/Jan/12/how-i-build-a-feature/
[2]: https://github.com/rednafi/reflections/issues/170
[3]: https://github.com/rednafi/reflections/tree/master/.github/workflows
[4]: https://rednafi.github.io/misc/automerge_dependabot_prs_on_github/
[5]: https://docs.github.com/en/actions/using-workflows/reusing-workflows
[6]: https://github.com/rednafi/reflections/blob/master/.github/workflows/ci.yml
[7]: https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions

1. [How I build a feature - Simon Willison](https://simonwillison.net/2022/Jan/12/how-i-build-a-feature/)
2. [Example issue that reflects the pattern explained here](https://github.com/rednafi/reflections/issues/170)
3. [Worflows directory of this blog](https://github.com/rednafi/reflections/tree/master/.github/workflows)
4. [Automerge Dependabot PRs on GitHub](https://rednafi.github.io/misc/automerge_dependabot_prs_on_github/)
5. [Reusing workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)
6. [The main CI file of this blog](https://github.com/rednafi/reflections/blob/master/.github/workflows/ci.yml)
7. [Understanding GitHub Actions](https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions)
