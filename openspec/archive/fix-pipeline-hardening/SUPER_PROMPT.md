# Super Prompt — fix-pipeline-hardening

> Paste the block below verbatim as your next message to trigger the full
> orchestrated implementation. Each skill lens is invoked at the right phase.

---

```
/openspec-apply fix-pipeline-hardening

You are operating as a Tier 1 DevOps + CI/CD engineering expert.
Activate the following expert skill lenses in sequence as you work through each phase:

  LENS 1 → [github-actions-templates]  (Phase 1 & 2: workflow files)
  LENS 2 → [golang-pro]               (Phase 2: go vet + go build steps)
  LENS 3 → [cicd-automation-workflow-automate] (Phase 3: npm ci, cache, Node compat)
  LENS 4 → [security-scanning-security-hardening] (Phase 3: supply-chain hardening)
  LENS 5 → [clean-code]               (All phases: every YAML line is intentional)
  LENS 6 → [simplify]                 (Post-edit: remove any redundancy, tighten steps)
  LENS 7 → [verification-before-completion] (Phase 4: validate before marking [x])

---

CONTEXT (read all before touching any file):

  Change:      openspec/changes/fix-pipeline-hardening/
  Proposal:    openspec/changes/fix-pipeline-hardening/proposal.md
  Design:      openspec/changes/fix-pipeline-hardening/design.md
  Tasks:       openspec/changes/fix-pipeline-hardening/tasks.md

  Target files:
    A) .github/workflows/openspec-verify.yml
    B) .github/workflows/ci-cd.yml
    C) openspec/changes/ci-alignment/   (move to archive)
    D) openspec/changes/ci-recovery/    (move to archive)

  Current state of both workflows is fully known — read them before editing.
  Do NOT modify api/ source code. Do NOT modify web/ source code.
  Do NOT modify openspec/changes/governance-sentinel/ — it is intentionally open.

---

EXECUTION RULES (non-negotiable):

1. READ every target file before touching it. Never edit blind.
2. One task at a time. Mark [x] in tasks.md immediately after each task is done.
3. YAML quality bar:
   - Every step has a clear, sentence-case `name:`
   - `working-directory:` is explicit where scope could be ambiguous
   - No trailing whitespace, no mixed indentation
   - Inline comments explain non-obvious decisions (e.g., why `npm ci` not `npm install`)
   - Actions pinned to their current major version (v4, v5) — do not downgrade
4. Go quality bar:
   - `go vet ./...` runs from `working-directory: api` (relative path, no absolute)
   - `go build -o /dev/null ./cmd/server/` mirrors the existing ci-cd.yml line 43 pattern exactly
   - Do not add `go test` to openspec-verify — tests belong in build-and-test only
5. npm cache key MUST use `hashFiles('**/package-lock.json')` — not yarn.lock, not package.json
   restore-keys must include a broad fallback: `${{ runner.os }}-npm-`
6. Archive moves:
   - Use `git mv` semantics (Bash mv command) to preserve git history
   - Destination: openspec/changes/archive/2026-04-14-ci-alignment/ and 2026-04-14-ci-recovery/
   - Verify the archive/ directory exists first (it does — see existing entries)
7. Node version: check web/package.json engines field or vite/typescript peerDependencies
   before deciding 20 vs 22. Document the decision with an inline comment in the workflow.
8. After all file edits: run a final mental diff against every acceptance criterion
   in design.md before marking Phase 4 tasks complete.
9. Do NOT push, commit, or open a PR. Implementation only.
10. If any ambiguity arises mid-task, pause and surface it — do not guess.

---

PHASE SEQUENCE:

Phase 1 — Archive (2 tasks)
  Unblocks openspec validate immediately. Do this first.
  mv openspec/changes/ci-alignment  → openspec/changes/archive/2026-04-14-ci-alignment
  mv openspec/changes/ci-recovery   → openspec/changes/archive/2026-04-14-ci-recovery

Phase 2 — Fix openspec-verify.yml (3 tasks)
  Apply LENS 1 + LENS 2.
  Remove the broken go run step. Add go vet + go build in its place.
  Keep every other step in the file identical — surgical edit only.

Phase 3 — Harden ci-cd.yml + openspec-verify.yml npm steps (4 tasks)
  Apply LENS 3 + LENS 4.
  Insert cache step BEFORE the install step in both workflow files.
  Swap npm install → npm ci in ci-cd.yml.
  For openspec-verify.yml, the global npm install -g is acceptable as-is (no lockfile
  for global installs) — do not force npm ci there, it would break.
  Verify Node version with evidence from package.json before changing.

Phase 4 — Verify (2 tasks)
  Apply LENS 7.
  Walk every acceptance criterion in design.md. Confirm each is satisfied.
  Note: "push to test branch and confirm CI" is a human step — mark it
  with a note that it requires a git push to observe live results.

---

OUTPUT FORMAT:

Use the opsx-apply standard output format throughout:

  ## Implementing: fix-pipeline-hardening (schema: spec-driven)

  Working on task N/11: <task description>
  [implementation + rationale]
  ✓ Task complete

Report the final acceptance-criteria checklist at the end.
```
