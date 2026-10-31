---
name: ask-persona
description: Ask one DoulaCloud persona a question she can answer by using the running app ("can a Client see her next Visit?"). Runs the blind persona-walker against the local stack and returns her answer with screenshots. Args: persona slug or name, then the question.
---

Answer a question by putting one persona in front of the app. Use the recipe in `docs/agents/persona-walk.md`; this skill is its question branch, and differs only where it says so.

1. **Pick the persona.** Match the argument to a slug in `docs/personas/` by slug or by name. A name that is no persona's (a Client "36 weeks pregnant" is not one) maps to the closest archetype in `docs/personas/README.md`'s tables, and the answer names which persona stood in.
2. **Start the stack** (recipe step 1), with the seed on unless she signs up cold. A Client with no seeded account needs one made first, as that step says.
3. **Conditions**: the recipe's defaults unless the question is about one condition (offline, keyboard, a wide screen).
4. **Build the brief** (recipe step 3) with a `Question:` line holding the question as she would hear it, in place of the stages to walk. Keep the stages and Thinking lines: they tell her what she is in the app for.
5. **Spawn** `persona-walker` with `model: "opus"`, in the foreground. The evaluator does not run: a question walk produces an answer, not a friction log.
6. **Report** to the asker: her `## Answer` block, the persona who answered, the shots it cites, the walked SHA and the conditions profile. Close her pages and stop the stack.

An answer is what one scripted persona saw on one walk. It settles what the screen shows, and never what Clients or doulas want ([`docs/personas/README.md`](../../../docs/personas/README.md)).
