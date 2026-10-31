---
name: ask-persona
description: Ask a DoulaCloud persona (hannah, renata, maya, any of the nine, by name or slug) a question. Answers in her voice from what the repo records about her, citing the source of each claim; walks the running app as her when the records are silent and the screen could answer. Args: name, then the question.
---

Put one question to one persona. There are two branches: a **consult**, which is fast, reads the records and is always run first, and a **walk**, which is slow, uses the screen and runs only when the consult sends you there.

## Consult

1. Spawn her named agent (`hannah`, `camille`, `nadia`, `maya`, `renata`, `priya`, `lena`, `dee`, `tasha`) with `model: "opus"`, in the foreground, or `persona-consult` with the name when it is none of those. Pass the question, and the screen, wording or PR it is about, when the asker named one.
2. Relay its reply to the asker whole: her answer, its Sources, and its Nothing on record list. Keep the agent's id: a follow-up goes to the same agent by `SendMessage`, so she answers it with the conversation so far.

Stop here unless an item in Nothing on record says `walk it` and the asker wants it settled.

## Walk

The question goes to the blind `persona-walker`, which knows only its brief. Follow `docs/agents/persona-walk.md` steps 1 to 3, then:

1. Write the brief's `Question:` line in place of the stages to walk, keeping the stages and Thinking lines, which tell her what she is in the app for. Write the question as a task and a reading, never as a choice between wordings. "Find your Birth Plan. Who is your doula?" is right; "Would you rather see the Practice name or the doula's?" is not, because a choice offered to her is answered by the asker's framing, and a reading is answered by the screen. Give her the start URL she would arrive from, and leave the way to the screen for her to find.
2. Spawn `persona-walker` with `model: "opus"`, in the foreground. The evaluator does not run, because a question walk produces an answer and not a friction log.
3. Relay her `## Answer` block, the shots it cites, the walked SHA and the conditions profile. Close her pages and stop the stack.

Either answer is what the records hold or what one scripted persona saw on one walk: evidence to weigh, never a claim about what Clients or doulas want ([`docs/personas/README.md`](../../../docs/personas/README.md)).
