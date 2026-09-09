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

// ⚠️ This gate USED TO FAIL OPEN, and that is the bug this block exists to stop.
//
// When npm cannot reach its advisory endpoint it still prints valid JSON — an
// object shaped `{"error":{...}}` rather than a report. The old code parsed that
// happily, found no `vulnerabilities` key, and printed "audit:ci passed" with
// exit 0. A dependency audit that cannot run was therefore indistinguishable
// from one that ran and found nothing.
//
// That is not hypothetical: npm's bulk advisory endpoint was intermittently
// timing out on 2026-09-04, falling back to the `/security/audits/quick`
// endpoint being decommissioned, which answers 400. Reproduced by stubbing npm:
// the gate printed "audit:ci passed" and exited 0 against an error payload.
//
// So: retry transport failures, and if the report is not a real report, FAIL.
// A gate that could not run must never report as a gate that passed.

const ATTEMPTS = 3
const TRANSPORT_HINT = /audit endpoint returned an error|ECONNRESET|ETIMEDOUT|network timeout|audits\/quick|EAI_AGAIN|socket hang up/i

function runNpmAudit() {
  // stderr is captured, not discarded: npm reports transport trouble there, and
  // discarding it is what made this failure mode invisible in the first place.
  try {
    const stdout = execSync('npm audit --json --fetch-timeout=45000 --fetch-retries=0', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    })
    return { stdout, stderr: '' }
  } catch (err) {
    // `npm audit` exits non-zero whenever vulnerabilities exist, but still prints
    // the JSON report to stdout — parse that rather than treating it as failure.
    return { stdout: err.stdout ? err.stdout.toString() : '', stderr: err.stderr ? err.stderr.toString() : String(err.message || '') }
  }
}

// A genuine npm audit report always carries these. An error payload carries
// neither, which is exactly how a failed run gets caught instead of passing.
function isRealReport(r) {
  return !!r && typeof r === 'object' && !r.error && (r.vulnerabilities !== undefined || r.metadata !== undefined)
}

let report
for (let attempt = 1; attempt <= ATTEMPTS; attempt++) {
  const { stdout, stderr } = runNpmAudit()

  let parsed = null
  try {
    parsed = JSON.parse(stdout)
  } catch {
    parsed = null
  }

  if (isRealReport(parsed)) {
    report = parsed
    break
  }

  const detail = (parsed && parsed.error && (parsed.error.summary || parsed.error.detail || parsed.error.code)) || stderr.trim().slice(0, 200) || 'no audit report returned'
  const transport = TRANSPORT_HINT.test(detail) || TRANSPORT_HINT.test(stderr)

  console.error(`audit:ci — attempt ${attempt}/${ATTEMPTS} did not produce an audit report: ${detail}`)

  if (!transport || attempt === ATTEMPTS) {
    console.error('')
    console.error('audit:ci FAILED — `npm audit` did not return a usable report.')
    console.error('This is NOT a clean audit: the gate did not run, so it is reported as a')
    console.error("failure rather than a pass. If npm's advisory endpoint is degraded, re-run")
    console.error("once it recovers. Do NOT rebuild the lockfile because of npm's")
    console.error(`"Invalid package tree" message: that is the retired endpoint's generic 400 body.`)
    process.exit(2)
  }

  const backoffMs = 10000 * attempt
  console.error(`  retrying in ${backoffMs / 1000}s`)
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, backoffMs) // sleep, synchronously
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
