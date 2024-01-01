---
title: Planning palooza
date: 2024-01-01
tags:
    - Meta
---

When I started my career in a tightly-knit team of six engineers at a small e-commerce
startup, I was struck by the remarkable efficiency of having a centralized hub for all the
documents used for planning. We used a single Trello board with four columns—To-do, Doing,
Q/A, Done—where the tickets were grouped by feature tags. We'd leverage a dummy ticket as an
epic to sketch out the full plan for a feature and link related tickets to it. The rest of
the discussions took place in Slack or GitHub Issues.

The setup was rudimentary but stood the test of time. As we expanded into multiple teams,
each unit had its own board mirroring the original structure. Managers had a clear picture
of where to find stuff, everything was searchable from one spot, and communication impedance
was surprisingly low.

A few years down the line, I was fortunate enough to land gigs at larger companies with
bigger teams and more corporate structures. What caught me off guard was the chaotic state
of planning documents. They were scattered everywhere—RFCs[^1], ADRs[^2], Epics, Jira
Issues, Subtasks, Design Docs[^3], you name it. Often, a single team would juggle all these
formats to plan and record their work. I'm not claiming every workplace was like this, but
it's more common than I'd like to admit.

The fallout? Discussing a feature or onboarding new people became a pain since explaining
any part of the system required going down the rabbit hole of finding the concomitant entry
point and traversing its branches. More often than not, the documents weren't even linked
properly, so figuring out which RFCs, ADRs, Epics, or Jira Issues were associated with what
feature was a frustrating exercise. Also, they'd quickly go outdated since keeping all of
them up to date was a full-time job itself!

This cultural shift doesn't happen in a day. People, in general, love reading books, blogs,
or Hacker News discussions about the engineering practices in FAANG companies and mean well
when they try to slowly incorporate these insights into their current teams. But let this
osmosis continue for a few years without any oversight, and you'll end up in a labyrinth of
documents, encumbered by stiff structures and other enterprise fluff.

Sometimes I wonder if all these theatrics are actually necessary to do the job. Hundreds of
people work on OSS projects where GitHub Issues and Projects are used to coordinate work.
Just pressing `cmd + k` lets you find anything, anywhere, allowing immediate access to
feature designs without having to sift through a quagmire of documents in disparate
locations. The ability to access all documentation from a single place is not just
efficient; it's empowering, especially if it's housed alongside your code.

Another approach that works well in practice is having a single Jira board per team where an
Epic contains all the design decisions of a feature, and individual Task tickets under that
are linked to GitHub Issues. This ensures that project managers can have a bird's eye view
of everything without having to log into the code repository, and developers can easily
navigate back to the corresponding Task and Epic from the GitHub Issue with a single click.

Whatever the strategy may be, I find it incredibly hard to justify the necessity to fragment
these documents behind obscure names and waste time endlessly bikeshedding about whether
ADRs should be written before RFCs or vice versa.

Fin!

[^1]:
    [Companies Using RFCs or Design Docs and Examples of These - Gergely Orosz](https://blog.pragmaticengineer.com/rfcs-and-design-docs/)

[^2]: [Architectural Decision Records (ADRs)](https://adr.github.io/)
[^3]:
    [Design docs at Google](https://www.industrialempathy.com/posts/design-docs-at-google/)
