## ADDED Requirements

### Requirement: Lazy-Loaded Section Layout Stability

Where the frontend defers a section behind a loading placeholder, the placeholder and the resolved section SHALL occupy the same minimum height, so that resolving the section does not change page height.

This SHALL hold for **every** state the section can resolve into — including error, empty and sparsely-populated states — not only the state whose natural height happens to match the placeholder. A reservation that matches only the common case relocates a layout shift rather than removing it.

Where a reservation is deliberately waived for a viewport class, it SHALL be waived on both the placeholder and the resolved section together.

#### Scenario: Resolved section matches its placeholder

- **WHEN** a deferred section finishes loading and replaces its placeholder
- **THEN** the page height SHALL NOT change as a result of the swap
- **AND** content below the section SHALL NOT move

#### Scenario: Section resolves into an error or empty state

- **WHEN** the deferred section resolves into a state whose natural content is shorter than the reserved height
- **THEN** the section SHALL still honour the reserved minimum height
- **AND** the page SHALL NOT shrink when the placeholder is replaced
- **AND** any resulting empty area SHALL be space the placeholder already occupied, so nothing visible moves

#### Scenario: Reservation waived consistently

- **WHEN** the reservation is waived for a viewport class because no shift is possible there
- **THEN** it SHALL be waived for the placeholder and the resolved section alike
- **AND** neither SHALL impose a reserved height the other does not
