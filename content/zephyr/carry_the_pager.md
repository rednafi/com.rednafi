---
title: If you won't carry the pager, maybe don't push to mainline
date: 2026-05-30
slug: carry-the-pager
tags:
    - Essay
    - AI
    - LLMs
    - Culture
description: >-
  Drive-by AI changes break the shared model a team builds around its code, and the ICs end
  up cleaning up the mess. Why pushing to mainline should come with the pager.
---

![Gandalf blocking the Balrog, captioned "You shall not pass"][image_1]

In a funny turn of events, as AI spending spirals out of control, _tokenmaxxing_ is now
_considered harmful_.

Recently, [Fortune] reported that Uber burned through its entire 2026 AI coding budget in
four months, with every usage stat up and to the right. But COO Andrew Macdonald still can't
tie any of it to features users would actually notice:

> Maybe implicitly there's more that is getting shipped, but it's very hard to draw a line
> between one of those stats and 'Okay now we're actually producing like 25% more useful
> consumer features.'

So it seems like all that spending bought a lot of code and not much else. Maybe a ton of
debt too.

Meanwhile, the AI overlords keep rewriting the narrative. Who knew the public wouldn't love
being told their job's obsolete in the next x months?

A year ago, [Dario Amodei cried wolf]:

> AI could wipe out _half_ of all entry-level white-collar jobs - and spike unemployment to
> 10-20% in the next one to five years.

By February, [Boris Cherny added to the fire]:

> So I think at this point, it's safe to say that coding is largely solved. At least for the
> kinds of programming that I do, it's just a solved problem because Claude can do it.

Then the IPOs got close and the tone flipped. This month, [Dario caved first]:

> If you automate 90% of the job, then everyone does the 10% of the job. And the 10% kind of
> expands to be 100% of what people do and kind of 10-times their productivity.

[Sam Altman followed three weeks later]:

> I'm delighted to be wrong about this. I thought there would have been more impact on
> entry-level white-collar jobs being eliminated by now than has actually happened.

In the beginning, this was an obvious PR stunt to sell leadership the nirvana of the
one-person company, where the agents do everything and the profit splits between the CEO and
the model provider. Too bad, that hasn't happened yet.

---

Everyone and their moms are now slinging code to production, not because they want to but
because they're pushed to, and because AI apparently makes everyone an expert in everything.

I'm a heavy LLM user myself, and I'm constantly fascinated by what it lets me do. But saying
things like

- frontend engineering is cooked
- AI is coming for backend
- who needs infra people when the model is so good at wrangling k8s
- writing docs isn't a problem, I just generated twenty pages of slop for my twenty-line
  script
- PMs are useless and should be replaced
- EMs should code now, and the ones who don't ngmi

is incredibly stupid. It creates a hunger-games environment at work. And for what? So that
Dario's next prediction becomes a self-fulfilling prophecy as we tear each other apart?

The AI leaderboard and tokenmaxxing gamification pull all kinds of weird behavior out of
people. PMs are forced to push code instead of doing their actual jobs. EMs are buried under
their own responsibilities and still expected to sit at the center of every technical
problem on the team. The result is that everyone is running around like headless chickens
switching from one task to the next, from one demo to the next and accomplishing nothing.
But hey, we're productive at least.

And no one cares about the poor ICs. The deadlines and expectations are getting batshit
crazy.

_Just use AI, bro, and be 10x productive._

Coding is important, but it's like 20% of the work, and making it 10x faster won't make the
whole thing go brrr! Hello [Amdahl's law].

---

Here's what Marc Brooker said [on a podcast]:

> If you aren't doing it hands on, your opinion about it is very likely to be completely
> wrong.

He's talking about leadership making technical decisions, and without being somewhat
hands-on, it's impossible to form an opinion. But does everyone need to form an opinion on
everything? Of course not, and that's exactly why we have hierarchies. Saying AI will
steamroll that into something flat and egalitarian is naive.

Add non-technical people to the mix, and it doesn't take long for the whole thing to turn
into a castle of glass. This breaks the social structure of an engineering team in the name
of forced democratization. Now this might sound like gatekeeping. But if democratization
means throwing out the playbooks we've built over the last 50 years, then gatekeeping is
just quality control with an uglier name.

Expecting code from an EM leading an infra team is very different from expecting it from a
PM or a designer. I'm all for anyone trying to pitch in and survive this madness, regardless
of their role. But hauling agents onto a mission-critical codebase isn't the way to do it.
Go try merging something into the Linux kernel with zero context. The kernel maintainers
can't allow you to merge random stuff into mainline without breaking the universe. So why
would it be gatekeeping if merging PRs at your workplace demands the same bar?

PMs and designers have plenty to contribute, just not directly to the code that runs in
production. AI makes prototyping, mocking, and wireframing faster than ever, and anyone can
do that without disrupting the critical path.

What it won't do is turn you into the expert it took some guy ten years to become. I can't
replace an EM with Claude, and I don't want to. I want my PM and my EM to do their jobs so I
can do mine. Also, please don't turn the poor designers into vibecoding jarheads. Let them
explore the tools at their own pace and see what they can do with them. Trying to force
everyone into the same box, all contributing to the production codebase, is a leadership
failure.

---

Building software means building a collective mental model of how the system works and how
it's run, and most of that model never makes it into the code or the docs. Peter Naur's
[Programming as Theory Building] is as relevant as ever:

> For a program to retain its quality it is mandatory that each modification is firmly
> grounded in the theory of it.

Generating code is cheap. Operating it is where you learn the kinks of a deployed artifact.
You don't build that model without the elbow grease.

And operating it means carrying the pager. When you get paged, you thread through the
incident, hotfix if you can, write the post-mortem and the [CoEs], and bang your head
against the wall until the root cause finally gives.

You can't do that alongside a zillion other responsibilities just because there's now AI.
Unless you're in the full software development lifecycle, your mental model is probably
plain wrong. You can't prompt your way into a moderately complex system without putting in
the work.

So if you haven't done that work and aren't ready to, your clanker-generated PR probably
does more harm than good. Anyone can prompt an LLM and get that PR, and someone from the
trenchline can do it even better. So it's questionable whether this whole song and dance is
a good use of anyone's time.

---

A drive-by contribution from out-of-band folks breaks the collective mental model. The
author never learned how the code is written or how it fails, and neither did the clanker
that wrote it. So often you get code that's locally correct but breaks some global invariant
the author never knew how to check.

Reviewing these slop PRs takes time, and the follow-up questions are futile, because the
author didn't do the work and can't address the concerns. Going back and forth is
pointless - they'll just feed your comments back to the model. So people avoid reviewing PRs
like that, and worse, they get LGTM'd and merged into mainline. Disaster!

And once it's merged, who owns the code? Usually not the drive-by author. They don't operate
the system, so when their change causes an outage, they're rarely the one firefighting. It's
just chucking the ball over the fence and calling it a day. So much for _"[you build it, you
run it]."_

One too many times, the ICs and trenchline workers get pulled in to clean up the mess -
triaging an incident caused by someone else's vibeslop, on code they never touched. It's
incredibly unfair, and a massive tax on people who are already overloaded.

_So unless you carry the pager, maybe your code shouldn't go to mainline._

Building anything useful takes all kinds of people. I don't like this clanker-driven,
homogeneous world we're heading toward. Being able to do more is welcome, but this lack of
accountability and the contest over who burns the most tokens probably won't get us there.

Oh look, the good ol' [Goodhart's law] is laughing at us again!

<!-- references -->
<!-- prettier-ignore-start -->

[fortune]:
    https://fortune.com/2026/05/26/uber-coo-ai-spending-tokens-claude-code/

[dario amodei cried wolf]:
    https://www.axios.com/2025/05/28/ai-jobs-white-collar-unemployment-anthropic

[boris cherny added to the fire]:
    https://www.lennysnewsletter.com/p/head-of-claude-code-what-happens

[dario caved first]:
    https://fortune.com/2026/05/26/sam-altman-dario-amodei-walking-back-ai-jobs-apocalypse-prophecies-ipo/

[sam altman followed three weeks later]:
    https://www.cityam.com/delighted-to-be-wrong-sam-altman-changes-tune-on-ai-job-apocalypse-fears/

[amdahl's law]:
    /maxims/#amdahl

[on a podcast]:
    https://www.youtube.com/watch?v=u3GjIXP9N0s

[image_1]:
    https://media.licdn.com/dms/image/v2/C5612AQEPXYvuzJ9baA/article-cover_image-shrink_720_1280/article-cover_image-shrink_720_1280/0/1520181432275?e=1781740800&v=beta&t=QWvuu2l7HXpwkuCuWqeYT4h55C6JpZRJTo5mac94Uhk

[programming as theory building]:
    https://pages.cs.wisc.edu/~remzi/Naur.pdf

[coes]:
    https://aws.amazon.com/blogs/mt/why-you-should-develop-a-correction-of-error-coe/

[you build it, you run it]:
    https://queue.acm.org/detail.cfm?id=1142065

[goodhart's law]:
    /maxims/#goodhart

<!-- prettier-ignore-end -->
