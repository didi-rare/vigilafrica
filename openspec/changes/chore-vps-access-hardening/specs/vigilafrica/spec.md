## ADDED Requirements

### Requirement: Deployment Authorisation

A credential that can deploy SHALL NOT thereby confer root on the host, and SHALL NOT be able to choose what is deployed.

The deploy principal SHALL be constrained to a fixed command. Any privileged helper it may invoke SHALL be root-owned and not writable by the deploy principal, as SHALL every manifest, allowlist, and compose artefact that helper reads — otherwise the constraint is decorative, because the principal can rewrite what the privileged step executes.

The release reference being deployed SHALL be validated on the host, against an authority the deploy credential cannot modify. Validation performed only on the CI runner is a workflow-integrity control and SHALL NOT be relied upon as a compromise control, because a stolen credential never executes the workflow.

#### Scenario: A deploy credential cannot obtain a shell

- **WHEN** the holder of a deploy credential connects and requests an interactive shell, a subsystem, port forwarding, or any command other than the fixed deploy command
- **THEN** the connection SHALL be refused
- **AND** an unexpected `SSH_ORIGINAL_COMMAND` SHALL be rejected rather than passed through
- **AND** data supplied on standard input SHALL NOT be executed

#### Scenario: A deploy credential cannot deploy an unapproved artefact

- **WHEN** a deploy request names a release reference or image digest that the release authority has not approved
- **THEN** the host SHALL refuse the deployment
- **AND** this SHALL hold when the request is made directly over SSH, bypassing CI entirely
- **AND** the approval record SHALL NOT be writable by the deploy principal

#### Scenario: The deploy host verifies who it is talking to, and vice versa

- **WHEN** a deployment connects to the host
- **THEN** the host key SHALL be verified against a pinned value
- **AND** a mismatched host key SHALL fail the deployment rather than emit a warning and continue

### Requirement: Environment Isolation

Staging and production SHALL NOT share a host principal, a credential, or a writable path.

Where production deployment is gated on human approval, that gate SHALL constitute a real boundary at the host level. A shared account defeats it, because reaching the ungated environment is then sufficient to reach the gated one.

#### Scenario: A staging credential cannot affect production

- **WHEN** the staging deploy credential is used against production paths, services, or volumes
- **THEN** the attempt SHALL fail
- **AND** the converse SHALL also hold for the production credential against staging

#### Scenario: A superseded principal is retired, not merely superseded

- **WHEN** per-environment deploy principals replace a shared one
- **THEN** the previous principal's authorised keys, privilege-escalation entries, group memberships, and file ownership SHALL be removed or transferred
- **AND** the previous credential SHALL be demonstrated to reach neither environment

### Requirement: Client IP Resolution

The system SHALL resolve the client IP address through a single code path, and SHALL honour forwarded headers only when the immediate peer is a trusted proxy.

The set of trusted peers SHALL be derived from the measured address of the reverse proxy, not from a broad private range that also contains untrusted workloads.

#### Scenario: Forwarded headers from an untrusted peer are ignored

- **WHEN** a request carrying `X-Forwarded-For` or `X-Real-IP` arrives from a peer that is not a configured trusted proxy
- **THEN** the system SHALL use the peer address as the client IP and disregard the headers
- **AND** this SHALL hold on every endpoint that resolves a client IP, including geolocation as well as rate limiting

#### Scenario: A co-resident container cannot forge a client identity

- **WHEN** a workload other than the reverse proxy can reach the API over a shared network
- **THEN** it SHALL NOT be able to present itself as a trusted proxy
- **AND** it SHALL NOT be able to set the client IP used for rate limiting or geolocation

#### Scenario: Rate limiting keys per client, and its failure is detectable

- **WHEN** requests arrive from two distinct client addresses through a trusted proxy
- **THEN** they SHALL be accounted to distinct rate-limit buckets
- **AND** a regression collapsing all clients into one bucket SHALL be detectable by an automated check
- **AND** that check SHALL NOT depend on the observing party having only one source address

### Requirement: Backup Recoverability

A backup SHALL be demonstrated to restore. An untested archive SHALL NOT be counted as a backup.

Backups SHALL capture every database in the cluster and the cluster-wide roles required to restore them, SHALL be stored off the host they protect, and SHALL fail loudly rather than produce a plausible-looking partial archive.

#### Scenario: A partial capture fails loudly

- **WHEN** any stage of the backup pipeline fails
- **THEN** the job SHALL report failure
- **AND** SHALL NOT leave behind an archive that appears successful
- **AND** an archive that is empty or implausibly small SHALL be treated as a failure

#### Scenario: Recovery is proven, not assumed

- **WHEN** recoverability is asserted
- **THEN** an archive SHALL have been restored into a scratch database
- **AND** representative row counts SHALL have been compared against the source
- **AND** this SHALL cover every database in the cluster, not only the primary application database
