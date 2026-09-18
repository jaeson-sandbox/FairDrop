# .agents/

This directory mirrors `.claude/skills/` exactly. The two trees are byte-identical and are
kept that way on purpose: `.claude/` is what Claude Code reads, and `.agents/` is the
tool-neutral location other agent runtimes look for by path, without naming it in any config
file here.

That means nothing in this repository references `.agents/` by name, and grepping for it finds
nothing — which makes it look abandoned when it is not. Hence this note.

If you are working on the skills themselves, change one tree and copy it to the other, or the
next agent to read the mirror will act on a stale copy. If you ever establish that no runtime
needs this path, deleting it removes roughly a quarter of the repository's tracked files.

The skills come from [BMAD](https://github.com/bmad-code-org/BMAD-METHOD); `_bmad/` holds the
installed module config and `_bmad-output/` holds everything they produced.
