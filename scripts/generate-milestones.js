const fs = require('node:fs');
const path = require('node:path');

const repoRoot = path.resolve(__dirname, '..');
const configPath = path.join(repoRoot, 'milestones.config.json');
const outputPath = path.join(repoRoot, 'web', 'src', 'data', 'milestones.json');
const roadmapPath = path.join(repoRoot, 'openspec', 'specs', 'vigilafrica', 'roadmap.md');

function fail(message) {
  throw new Error(message);
}

function readJson(filePath) {
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (error) {
    fail(`Unable to read JSON from ${path.relative(repoRoot, filePath)}: ${error.message}`);
  }
}

function ensureValidConfig(config) {
  if (!config || !Array.isArray(config.milestones)) {
    fail('milestones.config.json must contain a top-level "milestones" array.');
  }

  const seenVersions = new Set();

  for (const milestone of config.milestones) {
    if (!milestone || typeof milestone.version !== 'string' || typeof milestone.label !== 'string') {
      fail('Each milestone entry must include string "version" and "label" fields.');
    }

    if (seenVersions.has(milestone.version)) {
      fail(`Duplicate milestone version found in config: ${milestone.version}`);
    }

    seenVersions.add(milestone.version);
  }
}

function normalizeStatus(statusCell) {
  const normalized = statusCell.toLowerCase();

  if (normalized.includes('complete')) {
    return { active: false, complete: true };
  }

  if (normalized.includes('active') || normalized.includes('in progress')) {
    return { active: true, complete: false };
  }

  return { active: false, complete: false };
}

function readRoadmapStatuses() {
  if (!fs.existsSync(roadmapPath)) {
    fail(`Missing roadmap source at ${path.relative(repoRoot, roadmapPath)}`);
  }

  const roadmap = fs.readFileSync(roadmapPath, 'utf8');
  const lines = roadmap.split(/\r?\n/u);
  const statuses = new Map();

  for (const line of lines) {
    if (!line.startsWith('| v')) {
      continue;
    }

    const cells = line
      .split('|')
      .map((cell) => cell.trim())
      .filter(Boolean);

    if (cells.length < 4) {
      continue;
    }

    statuses.set(cells[0], normalizeStatus(cells[3]));
  }

  return statuses;
}

function main() {
  const config = readJson(configPath);
  ensureValidConfig(config);

  const roadmapStatuses = readRoadmapStatuses();

  const milestones = config.milestones.map((milestone) => {
    const status = roadmapStatuses.get(milestone.version);

    if (!status) {
      fail(`Milestone ${milestone.version} is missing from openspec/specs/vigilafrica/roadmap.md`);
    }

    return {
      label: milestone.label,
      active: status.active,
      complete: status.complete,
    };
  });

  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, `${JSON.stringify(milestones, null, 2)}\n`, 'utf8');

  console.log(`Generated ${path.relative(repoRoot, outputPath)}`);
  for (const milestone of milestones) {
    console.log(`- ${milestone.label}: active=${milestone.active}, complete=${milestone.complete}`);
  }
}

main();
