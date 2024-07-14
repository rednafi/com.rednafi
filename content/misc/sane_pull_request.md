---
title: The sane pull request
date: 2024-07-14
tags:
    - Git
---

One of the reasons why I'm a big advocate of rebasing and cleaning up feature branches, even
when the changes get squash-merged to the mainline, is that it makes the PR reviewer's life
a little easier. I've written about my rebase workflow before[^1] and learned a lot from the
Hacker News discussion[^2] around it.

While there's been no shortage of text on why and how to craft atomic commits[^3], I often
find those discussions focus too much on VCS hygiene, and the main benefit gets lost in the
minutiae. When working in a team setup, I've discovered that individual commits matter much
less than the final change list.

Also, I find some of the prescriptive suggestions for easier review, like keeping the PR
under ~150 lines, ensuring that the tests pass in each commit, and tidying the commits to be
strictly independent, quite cumbersome. Stacked PRs[^4] sometimes help to make large changes
a bit more tractable, but that comes with a whole set of review-conflict-feedback
challenges. So this piece will mainly focus on making large PRs a wee bit easier to work
with.

Here's a quick rundown of the things I find useful to make reviewing the grunt work of pull
requests a bit more tractable. I don't always strictly follow them while doing personal or
OSS work, but these steps have been helpful while working on a large shared repo at work.

-   Avoiding the temptation to lump tangentially related changes into a PR to speed things
    up.

-   Having a ton of fragmented commits makes filtering useless when navigating the PR diff
    in a platform like GitHub. I really like to filter diffs on GitHub, but it wouldn't be
    useful if the commits are all over the place.

    ![commit filter][image_1]

-   To make diff filtering better, I often rebase my feature branch after a messy
    development workflow and divide the changes into a few commits clustered around the core
    implementation, tests, documentation, dependency upgrades, and occasional refactoring.

-   Rebasing all the changes into a single commit is okay if the change is small, but for
    bigger changes, this does more harm than good.

-   I've rarely spent the time to ensure that the individual commits are perfect[^5]—in the
    sense that they're complete with passing tests or documentation. As long as the complete
    change list makes sense as a whole, it's good enough. YMMV. The main goal is to make
    sure the diff makes sense to the person reviewing the work.

-   Annotated comments from the author on the PR are great. I wish they'd take up less space
    and there was a way to collapse them individually.

    ![annotated comments][image_2]

-   Each PR must be connected to either an Issue or a Jira ticket, depending on how the team
    works.

-   Adding context, screenshots, gifs, and videos to the PR description makes things so much
    easier for me when I do the review. Being able to see that the changes work as intended
    without running the code has its benefits.

    ![feature preview][image_3]

-   Keeping the PR in draft state until it's ready to be reviewed. I'm not a fan of getting
    a notification to review some work only to find that it's not ready yet.

[^1]: [I kind of like rebasing](/misc/on_rebasing)

[^2]: [HN discussion on my rebasing workflow](https://news.ycombinator.com/item?id=40742628)

[^3]:
    [Make atomic git commits](https://www.aleksandrhovhannisyan.com/blog/atomic-git-commits/)

[^4]:
    [In praise of stacked PRs](https://benjamincongdon.me/blog/2022/07/17/In-Praise-of-Stacked-PRs/)

[^5]: [The perfect commit](https://simonwillison.net/2022/Oct/29/the-perfect-commit/)

[image_1]: https://github.com/user-attachments/assets/dfaafa0a-efff-4216-800d-64715ff65ea6
[image_2]: https://github.com/user-attachments/assets/1c33bdba-aaab-4979-a57b-5dc6f84d88da
[image_3]: https://github.com/user-attachments/assets/c308f9a5-ea0b-4e76-a85c-bd508ecc9940
