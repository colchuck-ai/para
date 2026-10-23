# Section drafting process

How each section of `spec.md` is written. Read this first when resuming the work in a new session.

## Priorities

**Reader clarity, and less is more.** Every section is written for a reader who has only the
spec: a person meeting para for the first time, or an AI agent dropped into a tree. Keep it simple.
The shortest section that is complete wins.

## Context to load when resuming

- `spec.md`: the outline. Each section's one sentence is its charter.
- `plans/clean-room/plan.md`: the decisions (D1 onward) and open questions.
- `plans/clean-room/research/`: research notes behind specific chapters.
- `para-design-v4.md` (on `main` until Phase 2, and always at the `poc-final` tag): the PoC design,
  background only. Where it disagrees with a decision, the decision wins.
- `bd show para-a1b.2`: the Phase 1 issue.

## The loop

1. **Go.** The human says go, naming a section or taking the next one in outline order.
2. **Draft.** Write the section in `spec.md` under its header, from its charter and the decisions.
3. **Humanize.** Run the draft through the humanizer skill.
4. **Review, round 1.** Three subagents review the draft in parallel, one lens each (below). They
   report findings only and never edit.
5. **Fix.** Check each finding and dismiss any that do not hold up, saying why. Fix every reject,
   blocker and major. Fix a minor only if the fix is clean and small.
6. **Review, round 2.** The same three lenses review the revised draft.
   - Only minors left: fix the clean ones, humanize the changed text, then go to human review.
   - Any reject, blocker or major left: stop and escalate to the human, who decides whether to
     continue.
7. **Human review.** The human reads the section and gives feedback; iterate until approved.
8. **Commit.** One commit per approved section.

At most two review rounds per section.

## Severities

| Severity | Meaning | Action |
| --- | --- | --- |
| Reject | The approach is wrong; the section needs rethinking, not editing | Must fix |
| Blocker | Contradicts a decision, states something false, or omits what the charter requires | Must fix |
| Major | A gap or ambiguity that would mislead a reader or an implementer | Must fix |
| Minor | Wording, ordering or small clarity issues | Fix if clean and small |

Each finding gives: location, severity, the problem, and a suggested fix.

## Review lenses

The reader-clarity lens carries the most weight.

1. **Reader clarity and less is more** (primary). Could a newcomer, human or agent, understand it
   with only the spec? Flags undefined terms, hidden assumptions, sentences with two readings,
   explanation mixed into rules, and anything repeated, padded or unnecessary.
2. **Consistency and scope.** Agrees with every decision and every other section's charter; stays
   inside its own charter and covers all of it; uses terms as the glossary defines them; carries
   nothing the decisions reversed (paths as addresses, skills, `CLAUDE.md` generation, `resource`
   as a kind).
3. **Implementation and edge cases.** Read as the engineer who must build and test it. Flags
   statements too vague to test, missing edge cases (empty tree, merge collision, hand edit,
   symlink), unhandled failures, security rules (path climbing, label loosening), and examples
   that disagree with each other or with Appendix A.

## Progress

Track the current section in `para-a1b.2`'s notes (`bd update para-a1b.2 --append-notes "..."`), so
a new session knows where to pick up.
