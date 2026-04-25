## ADDED Requirements

### Requirement: Isolated Demo Environment
The system SHALL provide a Docker Compose configuration that starts a demo instance of VigilAfrica with a pre-seeded database, no live ingestion, and no dependency on EONET availability.

#### Scenario: Demo environment starts from a single command
- **WHEN** a contributor runs `docker compose -f docker-compose.demo.yml up -d`
- **THEN** the API SHALL be reachable at `http://localhost:8080/health`
- **AND** the response SHALL include `"status":"ok"`
- **AND** `GET /v1/events` SHALL return at least 5 curated events without any EONET network call

#### Scenario: Demo data persists on restart
- **WHEN** the demo environment is stopped and restarted
- **THEN** all previously seeded events SHALL still be present
- **AND** no additional events SHALL have been ingested (live ingestor is disabled)

#### Scenario: Demo environment is documented for contributors
- **WHEN** a contributor reads `DEMO.md`
- **THEN** they SHALL be able to stand up the demo environment locally in under 30 minutes following only those instructions
- **AND** `DEMO.md` SHALL be linked from `CONTRIBUTING.md`

### Requirement: Demo Visual Record
The repository SHALL contain a screenshot and animated GIF demonstrating the running application state so README readers can evaluate the product without running it locally.

#### Scenario: Screenshot committed to repository
- **WHEN** a user views the repository on GitHub
- **THEN** at least one screenshot SHALL be present at `docs/screenshots/`
- **AND** the screenshot SHALL show the map with event markers visible

#### Scenario: Demo GIF committed to repository
- **WHEN** a user reads `README.md`
- **THEN** a demo GIF SHALL be embedded or linked
- **AND** the GIF SHALL be no longer than 30 seconds in duration
- **AND** the GIF file SHALL be no larger than 5 MB
