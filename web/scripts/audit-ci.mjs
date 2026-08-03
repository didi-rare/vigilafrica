#!/usr/bin/env node
// Web dependency audit gate. Replaces a bare `npm audit --audit-level=moderate`
// so we can carry a *narrow, documented* allowlist for advisories that are both
// (a) not reachable in this app AND (b) not cleanly fixable by a bump — the two
// conditions that make ADR-008's "bump, never suppress" impossible to satisfy.
//
// Everything else still fails the build. An allowlist entry must name the GHSA,
// the reason it is unreachable, why it can't be bumped, and a review date; the
// gate WARNS (and you should delete the entry) once the advisory stops being
// reported, so exceptions can't quietly outlive their justification.
//
// Fails CI on any advisory at moderate+ whose GHSA is not on the allowlist.

import { execSync } from 'node:child_process'

const ALLOWLIST = [
  // Empty. The react-router RSC-CSRF entry (GHSA-qwww-vcr4-c8h2) was removed on
  // 2026-08-03 once its own documented exit condition was met — react-router
  // published 8.3.0 (2026-07-22), outside the vulnerable 7.12.0–8.2.0 range.
  // The fix was a react-router-dom -> react-router v8 move; see chore-react-router-8.
]

const SEVERITY_RANK = { info: 1, low: 2, moderate: 3, high: 4, critical: 5 }
const THRESHOLD = SEVERITY_RANK.moderate // mirror the previous --audit-level=moderate

let report
try {
  report = JSON.parse(execSync('npm audit --json', { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }))
} catch (err) {
  // `npm audit` exits non-zero whenever vulnerabilities exist, but still prints
  // the JSON report to stdout — parse that rather than treating it as failure.
  if (!err.stdout) {
    console.error('audit:ci — could not run `npm audit --json`:', err.message)
    process.exit(2)
  }
  report = JSON.parse(err.stdout.toString())
}

const allowed = new Map(ALLOWLIST.map((a) => [a.ghsa, a]))
const seenGhsa = new Set()
const offending = new Map() // ghsa -> { severity, title, pkg }

for (const [pkg, vuln] of Object.entries(report.vulnerabilities || {})) {
  for (const via of vuln.via || []) {
    if (typeof via !== 'object' || !via.url) continue // string via = flagged transitively; keyed off its leaf GHSA
    const match = /GHSA-[a-z0-9-]+/i.exec(via.url)
    if (!match) continue
    if ((SEVERITY_RANK[via.severity] || 0) < THRESHOLD) continue
    const ghsa = match[0]
    seenGhsa.add(ghsa)
    if (!allowed.has(ghsa)) offending.set(ghsa, { severity: via.severity, title: via.title, pkg })
  }
}

// Transparency: report every allowlisted advisory, and flag stale ones.
for (const entry of ALLOWLIST) {
  if (seenGhsa.has(entry.ghsa)) {
    console.log(`ALLOWED  ${entry.ghsa} (${entry.package}) — review by ${entry.reviewBy}`)
  } else {
    console.log(`STALE    ${entry.ghsa} (${entry.package}) is no longer reported — delete it from the allowlist.`)
  }
}

if (offending.size > 0) {
  console.error(`\n${offending.size} advisory(ies) at moderate+ are NOT allowlisted:`)
  for (const [ghsa, info] of offending) {
    console.error(`  ${ghsa} [${info.severity}] ${info.pkg} — ${info.title}`)
  }
  console.error('\nFix by bumping the dependency (ADR-008). Only allowlist if the advisory is both unreachable AND unpatchable, with a documented rationale + review date.')
  process.exit(1)
}

console.log(`\naudit:ci passed — no un-allowlisted advisories at moderate+ (${ALLOWLIST.length} documented exception(s)).`)
