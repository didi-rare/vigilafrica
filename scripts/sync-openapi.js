#!/usr/bin/env node
/**
 * sync-openapi.js
 *
 * Copies the OpenAPI spec from its source of truth in openspec/ into the
 * Go handlers package where it is embedded via //go:embed.
 *
 * Source of truth : openspec/specs/vigilafrica/openapi.yaml
 * Embedded copy   : api/internal/handlers/openapi.yaml
 *
 * Run manually : node scripts/sync-openapi.js
 * Run via npm  : npm run sync:openapi
 * Runs automatically before every build via the prebuild hook.
 */

const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const src  = path.join(root, 'openspec', 'specs', 'vigilafrica', 'openapi.yaml');
const dest = path.join(root, 'api', 'internal', 'handlers', 'openapi.yaml');

if (!fs.existsSync(src)) {
  console.error(`sync-openapi: source not found: ${src}`);
  process.exit(1);
}

fs.copyFileSync(src, dest);
console.log(`sync-openapi: copied openapi.yaml → api/internal/handlers/openapi.yaml`);
