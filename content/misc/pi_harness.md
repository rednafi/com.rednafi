---
title: "The irrational effectiveness of the Pi harness"
slug: pi-harness
date: 2026-09-05T00:00:00+02:00
description: >-
    A month of coding with Pi, its small agent loop, and the skills and extensions I've
    added to it.
tags:
    - LLM
    - CLI
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /misc/pi-harness/
atUri: ""
---

![Pi running in Ghostty with a read tool call][image_1]

A few months back, Pi's creator Mario Zechner gave a talk on [developing Pi and slowing
things down]. It resonated with me so much that I wanted to try out the harness. I'm also
getting increasingly wary of how bloated Claude Code and Codex have gotten over time.

I have no idea what these harnesses are putting into the system prompt or how that's
affecting the models' responses. I also have no clue about how their tool calling works or
what search engine they're using while searching for something. This opaque nature, along
with the sweeping changes they make to their system instructions every now and then, makes
it difficult to build a reliable workflow. Models are non-deterministic enough. I really
don't want my harness to add to it.

Pi is a tiny coding harness written in TypeScript. By default, it gives the model four
tools: `read`, `write`, `edit`, and `bash`. Ostensibly, that's all you need for most coding
work. The core is small enough that you can read the important parts in one sitting. The
repo has more packages than this, but these are the four salient ones, and they're quite
pleasant to read:

```txt
packages/
|-- ai/           # model APIs, streaming, and token usage
|-- agent/        # agent loop and tool execution
|-- tui/          # terminal rendering and keyboard input
`-- coding-agent/ # CLI, sessions, and extensions
```

Its [agentic loop] boils down to this:

```ts
while (true) {
    const response = await callModel(messages, tools)
    messages.push(response)

    if (response.stopReason === "error" || response.stopReason === "aborted") {
        break
    }

    const calls = response.content.filter((part) => part.type === "toolCall")
    if (calls.length === 0) {
        break
    }

    const results = await executeTools(calls)
    messages.push(...results)
}
```

The model requests tool calls and Pi runs them. The results go back into the conversation.
This repeats until the model replies without requesting another tool. The actual loop also
handles queued user messages and emits events for the UI. The above sketch leaves those out,
along with tool validation and error handling.

Pi's [system prompt] and [tool definitions] together come in at around 1,000 tokens. In
contrast, the [Codex system prompt] that ships with the CLI is about 4,400 tokens before you
count its tool definitions. Claude Code isn't open source, but people have [reconstructed
its prompts], and the built-in tool descriptions alone add up to several thousand tokens.
There is no benchmark that proves these huge system prompts make the output measurably
better. But they make the sessions costlier from the get-go for sure.

Pi doesn't bundle extra skills, subagents, or MCP support. If you need them, you can ask Pi
to write an extension or install someone else's. Pi knows its internals, and writing an
extension is such a pleasant experience. You can literally one-shot most of the simple
extensions with any capable model.

I really like this opt-in approach. Don't add garbage you think I need that I actually
don't. Let me pick my own.

I'm also skeptical of installing random skills and extensions from the internet. My local
setup has a few I've written or adapted:

- [skill:whip]: tames the insufferable text that LLMs generate. I copied the content of
  [tropes.fyi] into a skill and regularly use it to fix PR descriptions at work.
- [extension:web.ts]: lets Pi search DuckDuckGo and open the results as plain text. It tells
  the model to read the pages before answering factual questions.
- [extension:subagent]: delegates a task to a separate Pi process with its own context. I
  copied the code from Pi's [extension example]. I can run agents in parallel or pass one
  agent's findings to the next.
- [extension:welcome-logo.ts]: draws the block-letter Pi logo in the screenshot above. This
  is the first Pi extension I wrote. It replaces the startup header and uses the current
  theme's accent color.

I don't use `/goal` or `/plan` mode. I still babysit agents and read every line of their
output, so I haven't found the way `/goal` works particularly useful for my workflow. As for
planning, I just ask the model to dump the plan in a `plan.md` file.

Apart from these, I also installed [pi-mcp-adapter] for MCP access. Its bundled
`mcp-scripting` skill lets Pi combine MCP calls in JavaScript. That's the only third-party
extension I'm currently using. At some point, I'd like to dig into its implementation and
write my own MCP extension too.

My [Pi configuration] is in my dotfiles repo.

I thought Pi's bare-bones defaults would slow me down. But almost irrationally, I've used it
every day for the last 30 days and haven't felt the need to return to a more feature-packed
harness. It feels good to use a tool whose core I can keep in my head without flailing. The
TUI has been rock solid too. It hasn't crashed or frozen on me once. I haven't seen it
flicker while I'm working. Maybe that's because it's a frikkin TUI and [not a game engine!]

<!-- references -->
<!-- prettier-ignore-start -->

[developing pi and slowing things down]:
    https://www.youtube.com/watch?v=RjfbvDXpFls

[agentic loop]:
    https://github.com/earendil-works/pi/blob/main/packages/agent/src/agent-loop.ts

[system prompt]:
    https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/core/system-prompt.ts

[tool definitions]:
    https://github.com/earendil-works/pi/tree/main/packages/coding-agent/src/core/tools

[codex system prompt]:
    https://github.com/openai/codex/blob/main/codex-rs/models-manager/prompt.md

[reconstructed its prompts]:
    https://github.com/Piebald-AI/claude-code-system-prompts

[skill:whip]:
    https://github.com/rednafi/dotfiles/tree/main/dot_agents/skills/whip

[tropes.fyi]:
    https://tropes.fyi

[extension:web.ts]:
    https://github.com/rednafi/dotfiles/blob/main/dot_pi/agent/extensions/web.ts

[extension:subagent]:
    https://github.com/rednafi/dotfiles/tree/main/dot_pi/agent/extensions/subagent

[extension example]:
    https://github.com/earendil-works/pi/tree/main/packages/coding-agent/examples/extensions/subagent

[extension:welcome-logo.ts]:
    https://github.com/rednafi/dotfiles/blob/main/dot_pi/agent/extensions/welcome-logo.ts

[pi-mcp-adapter]:
    https://github.com/nicobailon/pi-mcp-adapter

[pi configuration]:
    https://github.com/rednafi/dotfiles/tree/main/dot_pi/agent

[not a game engine!]:
    https://x.com/trq212/status/2014051501786931427

<!-- pi running in ghostty -->
[image_1]:
    https://blob.rednafi.com/misc/pi-harness/cover-62823da84914.png

<!-- prettier-ignore-end -->
