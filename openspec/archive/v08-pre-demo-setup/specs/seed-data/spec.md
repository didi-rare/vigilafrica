## MODIFIED Requirements

### Requirement: Seed Dataset Coverage
The seed dataset at `api/db/seeds/` SHALL cover all countries currently supported by VigilAfrica (Nigeria and Ghana), enabling full local development and demo standup without EONET connectivity.

#### Scenario: Nigeria seed events are present
- **WHEN** the seed is applied to a fresh database
- **THEN** at least 5 synthetic Nigeria events SHALL be present
- **AND** events SHALL cover at least 3 Nigerian states
- **AND** events SHALL include both Flood and Wildfire categories

#### Scenario: Ghana seed events are present
- **WHEN** the seed is applied to a fresh database
- **THEN** at least 3 synthetic Ghana events SHALL be present
- **AND** events SHALL cover at least 3 Ghanaian regions
- **AND** events SHALL include at least one Flood event

#### Scenario: Seed is idempotent
- **WHEN** the seed script is applied more than once to the same database
- **THEN** the event count SHALL NOT increase on subsequent runs
- **AND** no duplicate `source_id` errors SHALL be raised
