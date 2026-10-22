#!/usr/bin/env node
// Dependency audit gate for BOTH npm trees: `web/` (the default) and the repo
// root (`--dir .`, run from the repo root by CI). One implementation and one
// allowlist, so an exception is reviewed once rather than drifting between two
// copies. Replaces a bare `npm audit --audit-level=moderate`
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
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

// `--dir <path>` selects which npm tree to audit; defaults to the current
// directory, so `npm run audit:ci` inside web/ behaves exactly as before.
const dirFlag = process.argv.indexOf('--dir')
const AUDIT_DIR = resolve(dirFlag !== -1 ? process.argv[dirFlag + 1] : '.')

// Each entry is excused ONLY when every condition holds — otherwise it fails
// the build like any other advisory:
//   ghsa      the advisory id
//   package   the package carrying it; the same GHSA on any other package is
//             NOT excused
//   devOnly   when true, every installed copy of `package` in the audited tree
//             must be `dev: true` in its package-lock.json. If the package ever
//             becomes a production dependency, the exception stops applying.
//   reviewBy  YYYY-MM-DD. After this date the entry stops applying and CI goes
//             red until someone re-reviews it — an exception cannot quietly
//             outlive its justification.
//   reason / whyNoBump   the ADR-008 rationale, required in prose.
//
// History: the react-router RSC-CSRF entry (GHSA-qwww-vcr4-c8h2) was removed on
// 2026-08-03 once its own documented exit condition was met — react-router
// published 8.3.0 (2026-07-22), outside the vulnerable 7.12.0–8.2.0 range.
const ALLOWLIST = [
  {
    ghsa: 'GHSA-vfj7-8cjw-p6xm',
    package: 'braces',
    devOnly: true,
    reviewBy: '2026-11-02',
    reason:
      'Stack-exhaustion DoS from deeply nested brace PATTERNS. braces is reached ' +
      'only through dev/CI tooling — root: @fission-ai/openspec -> fast-glob -> ' +
      'micromatch; web: build tooling, dev:true in the lockfile. Every pattern it ' +
      'ever expands comes from our own repository, so the worst case is someone ' +
      'with commit access crashing our own CI. Nothing in the chain ships to users.',
    whyNoBump:
      'No patched release exists: the advisory covers braces <= 3.0.3 and 3.0.3 is ' +
      'the latest published version (checked 2026-10-03). Every link above it is ' +
      'already at latest (openspec 1.14.0, fast-glob 3.3.3, micromatch 4.0.8) and ' +
      'still depends on braces. npm\'s own suggested fix is a DOWNGRADE of openspec ' +
      'from 1.x to 0.17.2, which drops fast-glob but does nothing for web/.',
  },
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
      cwd: AUDIT_DIR,
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
const offending = new Map() // ghsa -> { severity, title, pkg, why }

// Loaded lazily: only needed when a devOnly entry has to be checked.
let lockPackages
function lockfilePackages() {
  if (lockPackages === undefined) {
    const lock = JSON.parse(readFileSync(join(AUDIT_DIR, 'package-lock.json'), 'utf8'))
    lockPackages = lock.packages || {}
  }
  return lockPackages
}

const today = new Date().toISOString().slice(0, 10)

// Returns null when `entry` genuinely excuses this finding, or the reason it
// does not. Every refusal falls through to `offending`, i.e. fails the build.
function exceptionRefusal(entry, pkg, vuln) {
  if (entry.package !== pkg) return `allowlisted for ${entry.package}, but reported on ${pkg}`
  if (today > entry.reviewBy) return `allowlist entry expired on ${entry.reviewBy} — re-review it`
  if (entry.devOnly) {
    const nodes = vuln.nodes || []
    if (nodes.length === 0) return 'devOnly entry, but npm reported no install paths to verify'
    const prod = nodes.filter((n) => lockfilePackages()[n]?.dev !== true)
    if (prod.length > 0) return `devOnly entry, but installed as a non-dev dependency at ${prod.join(', ')}`
  }
  return null
}

for (const [pkg, vuln] of Object.entries(report.vulnerabilities || {})) {
  for (const via of vuln.via || []) {
    if (typeof via !== 'object' || !via.url) continue // string via = flagged transitively; keyed off its leaf GHSA
    const match = /GHSA-[a-z0-9-]+/i.exec(via.url)
    if (!match) continue
    if ((SEVERITY_RANK[via.severity] || 0) < THRESHOLD) continue
    const ghsa = match[0]
    seenGhsa.add(ghsa)
    const entry = allowed.get(ghsa)
    const refusal = entry ? exceptionRefusal(entry, pkg, vuln) : 'not allowlisted'
    if (refusal) offending.set(ghsa, { severity: via.severity, title: via.title, pkg, why: refusal })
  }
}

// Transparency: report every allowlisted advisory, and flag stale ones.
console.log(`audit:ci — auditing ${AUDIT_DIR}`)
for (const entry of ALLOWLIST) {
  if (!seenGhsa.has(entry.ghsa)) {
    console.log(`STALE    ${entry.ghsa} (${entry.package}) is not reported in this tree — delete it once no tree reports it.`)
  } else if (!offending.has(entry.ghsa)) {
    console.log(`ALLOWED  ${entry.ghsa} (${entry.package}) — review by ${entry.reviewBy}`)
  }
}

if (offending.size > 0) {
  console.error(`\n${offending.size} advisory(ies) at moderate+ are NOT allowlisted:`)
  for (const [ghsa, info] of offending) {
    console.error(`  ${ghsa} [${info.severity}] ${info.pkg} — ${info.title}`)
    console.error(`      ${info.why}`)
  }
  console.error('\nFix by bumping the dependency (ADR-008). Only allowlist if the advisory is both unreachable AND unpatchable, with a documented rationale + review date.')
  process.exit(1)
}

console.log(`\naudit:ci passed — no un-allowlisted advisories at moderate+ (${ALLOWLIST.length} documented exception(s)).`)
