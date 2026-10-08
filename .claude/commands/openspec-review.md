---
description: Automatically conducts a strict code review of a recently completed feature, scoring against standard docs, vulnerabilities, and spec alignment.
---
# /openspec-review

Triggered natively by the user (or during the `/openspec-archive` process) to enforce strict peer-review behavior by the AI agent on newly written code.

## Objectives
1. Verify the newly written code perfectly satisfies the acceptance criteria in the active `openspec/specs/[change-id].md`.
2. Cross-reference the code against `docs/standards/developers-react.md` and `docs/standards/developers-go.md`.
3. Actively scan for OWASP top-10 security vulnerabilities and hardcoded secrets.
4. Block archiving or merging if major standards are violated.

## Instructions
1. **Locate Code:** Identify the files recently modified by comparing the active spec's "Components to Touch" section with the real filesystem.
2. **Spec Alignment Check:** 
   - Does this piece of code satisfy the "What" and "Why" inside the spec?
   - Is any scope missing? Did the developer "forget" a required acceptance criteria?
3. **Standards Check:** 
   - Use `view_file` to read `docs/standards/developers-react.md` (if reviewing mobile code) or `docs/standards/developers-go.md` (if reviewing API code).
   - `developers-go.md` is the authoritative Go coding standard. 11 numbered sections: §1 Package Structure, §2 Configuration & Secrets, §3 Context Propagation, §4 Error Handling, §5 Repository Pattern & DB Access, §6 HTTP Handlers & Middleware, §7 Concurrency & Goroutine Lifecycle, §8 Logging & Observability, §9 Testing, §10 Dependencies & Modules, §11 Migrations & SQL.
   - `developers-react.md` is the authoritative React/TS coding standard. 15 numbered sections: §1 Project Layout, §2 TypeScript, §3 Component Design, §4 State Management, §5 Data Fetching, §6 Routing, §7 Styling, §8 Performance, §9 Accessibility, §10 Error Handling, §11 Forms, §12 Map/Geo Rendering, §13 Testing, §14 Dependencies, §15 Build & Env.
   - Every rule in both docs has a stable citation of the form `§N.M`.
   - When flagging a violation, **cite the specific rule number** (e.g. "violates §5.3 — user value interpolated into SQL string") so the contributor can look up the rationale and example directly. Generic "bad pattern" findings without a rule citation are insufficient.
   - Flag any structural violations (e.g. placing SQL logic directly in a Go HTTP handler instead of a repository per §5.1, or skipping memoization on a heavy React list).
4. **Security Check:**
   - Scan for hardcoded environment variables.
   - Scan for string-concatenated SQL queries (SQL Injection).
   - Scan for failure to wrap errors properly or using `panic` in Go.
   - Verify `expo-secure-store` is used for sensitive variables on mobile.
5. **Report Generation:** Provide the user with a highly structured markdown report of the findings. If it explicitly passes, prompt them to run `/openspec-archive`. If it fails, outline exactly what lines of code need fixing.
