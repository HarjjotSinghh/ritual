# Roadmap

Ordered by how much they change what the tool can tell you, not by effort.

## Next

**Semantic clustering.** Similarity is lexical today, so two prompts describing
the same work in entirely different words may not group. Local embeddings
(through an already-installed Ollama, never a hosted API) would fix the clearest
class of misses. Cohesion is already reported, so the improvement would be
measurable against `ritual eval` rather than asserted.

**Incremental scans.** A full read of a year of history takes seconds, not
minutes, but it re-reads everything. Fingerprinting files by size and
modification time would make `ritual scan` near-instant on the common case of
"what changed since yesterday".

**Skill drift reports.** `ritual` already detects that a workflow has a skill.
The more useful output is the diff: which steps the transcripts show that the
skill does not mention, and which of the skill's steps nobody has run in a
month.

## Later

**More agents.** Windsurf, Amp, Aider, Zed's agent, Antigravity. Amp is
server-backed and may never be readable locally; that is worth documenting
rather than attempting.

**Watch mode.** A background run that re-mines new sessions and surfaces new
candidates, without becoming something that runs unattended on a corpus this
sensitive. The design question is not technical.

**Team mode, carefully.** Several people doing the same undocumented procedure
is the strongest possible signal for writing it down. Doing this without
centralizing anybody's transcripts is the hard part, and the feature is not
worth having if it requires that.

## Deliberately not planned

**A hosted version.** Asking a developer to upload six months of agent
transcripts to a third party is asking them to upload their employer's source
code and their production credentials. See [docs/privacy.md](docs/privacy.md).

**Model-driven discovery.** A language model could find patterns the clustering
misses, and could equally invent them. The value of this tool is that every
suggestion can be checked against a transcript; a finding whose provenance is "a
model thought so" cannot be. The model stays where it belongs — improving prose
after the evidence is settled.
