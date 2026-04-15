# feature-dynamic-milestones

- [x] Review the active proposal, spec, workflow, and relevant app/build files.
- [x] Add `milestones.config.json` with milestone labels, ordering, and minimal matching metadata.
- [x] Add `scripts/generate-milestones.js` to derive milestone state from OpenSpec files.
- [x] Generate `web/src/data/milestones.json` from the script and commit the generated artifact.
- [x] Replace the hardcoded milestone array in `web/src/App.tsx` with a static data import.
- [x] Update build wiring in root scripts so milestone generation runs in the web build flow.
- [x] Update `.github/workflows/ci-cd.yml` to generate milestone data before building the web app.
- [x] Run verification: generator, lint, TypeScript build, and web build.
- [x] Perform a spec-alignment review and document assumptions/deviations in the final report.
