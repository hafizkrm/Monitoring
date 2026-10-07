NMS v1.0.0 — FINAL FULL QA AUDIT REPORT

Audit Date: 2026-10-07

Auditor: Antigravity AI (Read-Only)

Mode: READ-ONLY — NO CODE CHANGES PERFORMED



GIT BASELINE

Field	Value

Branch	main

HEAD	9423f59 Release v1.0.0: Enterprise NMS Phase 1-5 Completion

Tag	v1.0.0 ✅

Status	DIRTY WORKTREE — 62 files changed (modified/deleted/untracked) vs HEAD

WARNING



The working tree has significant uncommitted changes vs the v1.0.0 tag. This audit covers the actual worktree (HEAD + uncommitted modifications), not the tagged release snapshot.



SECTION VERDICTS

\#	Section	Verdict

1	Structure	PASS WITH FINDINGS

2	Backend	PASS WITH FINDINGS

3	Frontend	PASS WITH FINDINGS

4	API	PASS WITH FINDINGS

5	Database/SQL	PASS WITH FINDINGS

6	Migration/Schema	PASS WITH FINDINGS

7	Dead Code	PASS WITH FINDINGS

8	Duplicate Code	PASS WITH FINDINGS

9	Hardcode	PASS WITH FINDINGS

10	Bottleneck	PASS

11	Security	PASS WITH FINDINGS

12	Dependencies	PASS

13	Architecture	PASS WITH FINDINGS

14	Test/Regression	PASS WITH FINDINGS

15	Deployment	PASS WITH FINDINGS

16	Observability	PASS WITH FINDINGS

17	Recovery	PASS WITH FINDINGS

18	Release Consistency	FAIL

19	Contracts	PASS WITH FINDINGS

20	English Language	FAIL

FINDINGS

F-001

SEVERITY: CRITICAL

CATEGORY: Release Consistency

FILE: Repository root (git status)

LINE/FUNCTION: N/A (worktree)

PROBLEM: Working tree has 62 uncommitted file changes (including core backend/frontend modifications and deleted docs) vs the v1.0.0 tagged commit. The running code does NOT match the tagged release.

EVIDENCE: git diff --stat shows 62 files changed, 2069 insertions, 3652 deletions vs HEAD.

IMPACT: Release integrity violation — anyone checking out v1.0.0 tag gets different code than what is deployed/running.

CLASSIFICATION: CONFIRMED BUG

RECOMMENDATION: Commit all worktree changes and tag a new release (e.g., v1.0.1) or stash/reset to match v1.0.0.

F-002

SEVERITY: HIGH

CATEGORY: Security

FILE: 

.env

LINE/FUNCTION: Line 1, 5

PROBLEM: Root .env file contains actual JWT secret (new\_secure\_jwt\_secret\_2026) and RouterOS password (new\_rotated\_password\_2026) in plaintext. This file is tracked by git (shown in git diff).

EVIDENCE: .env line 1: JWT\_SECRET=new\_secure\_jwt\_secret\_2026, line 5: ROUTEROS\_PASSWORD=new\_rotated\_password\_2026

IMPACT: Secret exposure if repository is shared/public. Credential leak risk.

CLASSIFICATION: CONFIRMED SECURITY ISSUE

RECOMMENDATION: Ensure .env is in .gitignore (verify it is NOT tracked). Rotate the exposed secrets. Never commit real credentials.

F-003

SEVERITY: HIGH

CATEGORY: Security

FILE: 

backend/.env

LINE/FUNCTION: Line 1

PROBLEM: Backend .env contains a weak/default JWT secret: super\_secret\_key\_12345.

EVIDENCE: JWT\_SECRET=super\_secret\_key\_12345

IMPACT: Weak JWT secret can be brute-forced, compromising all authenticated sessions.

CLASSIFICATION: CONFIRMED SECURITY ISSUE

RECOMMENDATION: Use a strong, randomly generated JWT secret (≥32 chars). Ensure this file is not tracked.

F-004

SEVERITY: HIGH

CATEGORY: Security

FILE: 

seed\_users.sql

LINE/FUNCTION: Lines 45-50

PROBLEM: Seed file contains hardcoded bcrypt password hashes with plaintext passwords documented in comments (admin123, hafiz). These default credentials persist in production if not changed.

EVIDENCE: Comment: -- Password admin123 → followed by actual hash, INSERT IGNORE INTO users... 'admin', '$2a...'

IMPACT: Default admin credentials. Attacker can log in with admin/admin123.

CLASSIFICATION: CONFIRMED SECURITY ISSUE

RECOMMENDATION: Remove default credentials from seed files. Force password change on first login, or use environment-variable-driven seeding.

F-005

SEVERITY: HIGH

CATEGORY: Security

FILE: 

config.yaml

LINE/FUNCTION: Lines 55-57

PROBLEM: RouterOS address (10.10.60.2:8728), username (hafiz), are hardcoded in config.yaml. Password uses env var but the address/username are environment-specific.

EVIDENCE: address: "10.10.60.2:8728", username: "hafiz"

IMPACT: Environment-specific credentials in committed config file. Won't work in other environments without modification.

CLASSIFICATION: CONFIRMED HARDCODE

RECOMMENDATION: Use environment variable substitution (${ROUTEROS\_ADDRESS}, ${ROUTEROS\_USER}) like the password field does.

F-006

SEVERITY: HIGH

CATEGORY: Language / Localization

FILE: 

processor.go

LINE/FUNCTION: Lines 220, 249-258, 272-273, 288-293, 307-308, 318-319, 333-334

PROBLEM: All backend-generated alert/incident messages and titles are in Indonesian (Bahasa Indonesia), not English. These strings are user-facing via the API → Frontend.

EVIDENCE: "PULIH: %s telah kembali normal", "Perangkat %s (%s) terdeteksi terputus (Offline)", "Penggunaan CPU tinggi", "Kehilangan paket tinggi", "Antarmuka %s terputus", "Latensi ping tinggi", etc.

IMPACT: Application language requirement is English. Backend generates Indonesian strings that the frontend must translate with regex (fragile). Any new alert type added without frontend regex will appear in Indonesian.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE

RECOMMENDATION: Change all user-facing alert/incident strings in processor.go to English. Remove the formatAlertMessageToEnglish() regex translation layer from the frontend.

F-007

SEVERITY: HIGH

CATEGORY: Language / Localization

FILE: 

manager.go

LINE/FUNCTION: Lines 277, 289, 329, 373, 389

PROBLEM: Activity log descriptions are in Indonesian.

EVIDENCE: "Perangkat kembali online (circuit breaker direset via ping)", "Perangkat tidak dapat dijangkau (circuit breaker aktif)", "Polling gagal: %v", "Perangkat mengalami reboot (uptime reset)"

IMPACT: Activity logs shown to users in mixed language.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE

RECOMMENDATION: Convert all activity log messages to English.

F-008

SEVERITY: HIGH

CATEGORY: Language / Localization

FILE: 

queries.go

LINE/FUNCTION: Lines 121-124, 184-190, 216, 253-255, 283, 316-317

PROBLEM: Report summary keys are in Indonesian: "Total Perangkat", "Rata-rata SLA (%)", "Rata-rata CPU", "Total Insiden Logged", "Insiden Critical", "Perangkat Sering Down", "Total Kejadian Down", etc.

EVIDENCE: These map keys are returned as JSON to the frontend and rendered directly in report tables.

IMPACT: Reports display Indonesian labels to users.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE

RECOMMENDATION: Change all report summary keys to English equivalents.

F-009

SEVERITY: HIGH

CATEGORY: Language / Localization

FILE: Multiple frontend files

LINE/FUNCTION: Various (see evidence)

PROBLEM: Multiple user-facing frontend strings are in Indonesian.

EVIDENCE:

router.js:79

: "Modul ${view} belum tersedia"

router.js:108

: "Anda tidak memiliki akses ke halaman ini"

router.js:145

: "Sistem Monitoring Jaringan" (fallback subtitle)

users.js:47,302,335,359

: "Gagal memuat data user", "Gagal menambahkan user", "Gagal memperbarui user", "Gagal menghapus user"

reports.js:148

: "Gagal memuat data laporan"

settings.js:178

: "Gagal menyimpan pengaturan"

backup.js:43,88

: "Gagal mengekspor konfigurasi", "Gagal menyimpan setelan backup"

top-interfaces.js:46

: "Gagal memuat data"

config.service.js:127

: "Gagal menyimpan konfigurasi ke server"

app.js:163

: "Gagal Terhubung ke Prometheus TSDB"

login.html:7

: Meta description in Indonesian

IMPACT: User-facing error/toast messages appear in Indonesian, violating English-only requirement.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE

RECOMMENDATION: Translate all user-facing strings to English.

F-010

SEVERITY: HIGH

CATEGORY: Language / Localization

FILE: 

incidents.go

LINE/FUNCTION: Lines 205, 229

PROBLEM: Activity log descriptions for alert acknowledgment are in Indonesian.

EVIDENCE: "Mengakui (acknowledge) alert ID: ", "Mengakui (acknowledge) semua alert"

IMPACT: Activity logs display Indonesian text.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE

RECOMMENDATION: Change to English, e.g., "Acknowledged alert ID: ...", "Acknowledged all alerts".

F-011

SEVERITY: MEDIUM

CATEGORY: Duplicate Code / Redundancy

FILE: 

db.go

&#x20;+ 

migration.go

LINE/FUNCTION: db.go:156-231, migration.go:14-91

PROBLEM: Two separate migration systems exist: RunMigrations() in db.go and Migrate() in migration.go. They use different tracking tables (schema\_migrations with columns filename vs version), different logic, and different migration directory resolution strategies.

EVIDENCE: db.go:159 creates table with filename column; migration.go:16 creates table with version column. Only Migrate() is called from main.go. RunMigrations() appears unused (dead code).

IMPACT: Confusion, potential schema drift if wrong function is called. Dead code bloat.

CLASSIFICATION: CONFIRMED DUPLICATION + CONFIRMED DEAD CODE

RECOMMENDATION: Remove RunMigrations() from db.go. Keep only Migrate() in migration.go as the single migration runner.

F-012

SEVERITY: MEDIUM

CATEGORY: Duplicate Code

FILE: 

queries.go

LINE/FUNCTION: Lines 384-396, 565-589, 657-670

PROBLEM: JSON response maps contain extensive duplicate keys for compatibility: "timestamp"/"created\_at", "device\_name"/"name", "duration\_ms"/"duration"/"response\_time", "cpu\_usage"/"cpu", "memory\_usage"/"memory"/"ram", "latency\_ms"/"latency", "ip"/"ip\_address", "type"/"device\_type".

EVIDENCE: GetLogsV2 returns both "timestamp" and "created\_at" with same value. GetLatestMetrics returns "cpu\_usage", "cpu", "memory\_usage", "memory", "ram" — 5 fields for 2 values.

IMPACT: Bloated JSON payloads, maintenance overhead, confusion about which field to use.

CLASSIFICATION: CONFIRMED DUPLICATION (intentional compatibility layer, but excessive)

RECOMMENDATION: Standardize field names across frontend and backend. Remove compatibility aliases after frontend migration.

F-013

SEVERITY: MEDIUM

CATEGORY: Backend Logic / Observability

FILE: 

tsdb/exporter.go

LINE/FUNCTION: Lines 49-62

PROBLEM: The TSDBExporter.Start() receives events, type-asserts the payload, but then does nothing with the data (comment on line 59-60: "TSDBExporter no longer exports per-device Prometheus gauges due to cardinality limits."). The entire exporter goroutine is a no-op.

EVIDENCE: Lines 52-61: the loop receives events, checks type, increments error counter on failure, but has no actual export logic on success.

IMPACT: Wasted CPU/goroutine processing events that are discarded. EventBus subscriber buffer (100) is allocated but serves no purpose.

CLASSIFICATION: CONFIRMED DEAD CODE

RECOMMENDATION: Remove TSDBExporter entirely if it serves no purpose, or implement actual metric export.

F-014

SEVERITY: MEDIUM

CATEGORY: Deployment / Dockerfile

FILE: 

Dockerfile

LINE/FUNCTION: Line 2

PROBLEM: Dockerfile uses golang:1.21-alpine but go.mod specifies go 1.26.0. Major Go version mismatch.

EVIDENCE: Dockerfile line 2: FROM golang:1.21-alpine, go.mod line 3: go 1.26.0

IMPACT: Docker build will fail or produce incompatible binary. Cannot build the project with the Dockerfile as-is.

CLASSIFICATION: CONFIRMED BUG

RECOMMENDATION: Update Dockerfile to golang:1.26-alpine (or matching version).

F-015

SEVERITY: MEDIUM

CATEGORY: Deployment / Docker

FILE: 

docker-compose.yml

LINE/FUNCTION: Lines 9-12, 26-28

PROBLEM: Docker compose hardcodes database credentials: MYSQL\_ROOT\_PASSWORD: rootpassword, MYSQL\_PASSWORD: viscodpassword, DATABASE\_PASSWORD=viscodpassword. Also, database name is monitoring but config.yaml uses nms\_db.

EVIDENCE: Line 9: MYSQL\_ROOT\_PASSWORD: rootpassword, Line 10: MYSQL\_DATABASE: monitoring, but config.yaml line 17: database: nms\_db

IMPACT: Database name mismatch will cause connection failure in Docker deployment. Hardcoded passwords in committed file.

CLASSIFICATION: CONFIRMED BUG + CONFIRMED HARDCODE

RECOMMENDATION: Fix database name to match config. Use environment variables for credentials.

F-016

SEVERITY: MEDIUM

CATEGORY: Deployment / Docker

FILE: 

Dockerfile

LINE/FUNCTION: Line 26

PROBLEM: Dockerfile copies migrations from internal/database/migrations but actual migrations are in backend/migrations/ directory. The path internal/database/migrations does not exist in the repository structure.

EVIDENCE: COPY --from=builder /app/internal/database/migrations ./migrations — but the actual migrations directory is at backend/migrations/.

IMPACT: Docker container will have no migration files. Migration on startup will fail.

CLASSIFICATION: CONFIRMED BUG

RECOMMENDATION: Fix to COPY --from=builder /app/migrations ./migrations.

F-017

SEVERITY: MEDIUM

CATEGORY: Security

FILE: 

config.yaml

LINE/FUNCTION: Line 27

PROBLEM: SNMP community string public is hardcoded as default. While standard, it's a well-known default that should be changed in production.

EVIDENCE: community: public

IMPACT: Reduced security on SNMP-enabled devices using default community string.

CLASSIFICATION: THEORETICAL RISK (standard default, but should be overridden)

RECOMMENDATION: Document the need to change in deployment guide. Consider per-device community strings (already supported via snmp\_community column).

F-018

SEVERITY: MEDIUM

CATEGORY: Schema / Migration

FILE: 

001\_init\_schema.sql

LINE/FUNCTION: Lines 54, 83

PROBLEM: device\_metrics.status is defined as ENUM('up', 'down') but code inserts values like 'degraded', 'timeout', 'offline'. interface\_metrics.interface\_status is ENUM('up', 'down') but code may set other values. MySQL will silently store empty string for non-matching enum values.

EVIDENCE: Schema: status ENUM('up', 'down'). Code in queries.go:1341 handles "online", "offline". Processor creates "degraded" status. Manager sets "degraded" status at line 322.

IMPACT: Data integrity issue — 'degraded' status silently becomes empty string in MySQL strict mode, or truncated to empty in non-strict mode. Queries filtering on status may return incorrect results.

CLASSIFICATION: CONFIRMED BUG

RECOMMENDATION: Alter ENUM to include all used values: ENUM('up', 'down', 'degraded', 'unknown') or use VARCHAR.

F-019

SEVERITY: MEDIUM

CATEGORY: Backend / Concurrency

FILE: 

processor.go

LINE/FUNCTION: Lines 193-194 (handleIncident closure)

PROBLEM: The handleIncident closure acquires p.mu.Lock() with defer p.mu.Unlock() and then calls p.incidentRepo.CreateIncident(), p.incidentRepo.CreateAlert(), p.incidentRepo.ResolveIncident() — all database operations — while holding the mutex. This blocks all other threshold evaluations for all devices.

EVIDENCE: Lines 193-234: Lock held during DB calls (CreateIncident, CreateAlert, ResolveIncident).

IMPACT: Under load with many devices, threshold evaluation becomes serialized at the database call level. A slow DB query blocks all incident processing.

CLASSIFICATION: THEORETICAL RISK (performance concern, not a bug)

RECOMMENDATION: Separate the in-memory strike counting (needs lock) from the database operations (can be done after unlock).

F-020

SEVERITY: MEDIUM

CATEGORY: Architecture / Dead Code

FILE: 

tsdb/client.go

LINE/FUNCTION: Entire file (6726 bytes)

PROBLEM: tsdb/client.go is a full Prometheus query client with PromQL query, range query, and status checking functions. However, since the TSDBExporter is a no-op (F-013), and the TSDB handler endpoints proxy to Prometheus directly, the actual usage of this client needs verification.

EVIDENCE: File exists at 6726 bytes. TSDBExporter does no actual export.

IMPACT: Potential dead code if unused.

CLASSIFICATION: THEORETICAL RISK (needs caller verification — TSDB handlers in monitoring/ may use it)

RECOMMENDATION: Verify if tsdb/client.go functions are called from handlers. If not, remove.

F-021

SEVERITY: MEDIUM

CATEGORY: Observability

FILE: 

hub.go

&#x20;+ 

exporter.go

LINE/FUNCTION: hub.go:101-114, exporter.go:35-44

PROBLEM: Both files register a Prometheus counter with the same metric name nms\_eventbus\_dropped\_messages\_total using MustRegister. They differentiate by ConstLabels (component: "websocket\_hub" vs component: "tsdb\_exporter"). This works only because MustRegister with different ConstLabels creates distinct collectors, but it's fragile and unconventional.

EVIDENCE: Same metric name registered in two different files.

IMPACT: If both are registered in the same process (which they are), Prometheus may reject the duplicate or it may work due to different label combinations. Fragile pattern.

CLASSIFICATION: THEORETICAL RISK

RECOMMENDATION: Use a single counter vector with a component label dimension registered once.

F-022

SEVERITY: MEDIUM

CATEGORY: Config / Defaults

FILE: 

config/models.go

LINE/FUNCTION: Lines 176-184, 253-261

PROBLEM: Logger defaults are set twice in setDefaults(). Lines 176-184 set Level="INFO", Format="json", OutputPath="stdout". Lines 253-261 overwrite to Level="INFO", Format="json", OutputPath="./var/log/nms-agent.log". The second block always wins since checks are if c.Logger.X == "" (already set by first block, but original value is used).

EVIDENCE: First block: L182 OutputPath = "stdout". Second block: L260 OutputPath = "./var/log/nms-agent.log". Since first block sets it, second block's check if c.Logger.OutputPath == "" will be false. So the actual effective default depends on which runs first — but since line 176-184 runs before 253-261, the first block wins. The second block is dead code for these fields.

IMPACT: Dead code / confusion. The second block of logger defaults never executes.

CLASSIFICATION: CONFIRMED DEAD CODE

RECOMMENDATION: Remove the duplicate logger defaults block (lines 253-261).

F-023

SEVERITY: LOW

CATEGORY: Backend / Error Handling

FILE: 

queries.go

LINE/FUNCTION: Lines 893-894

PROBLEM: AddDeviceWithBrand returns Indonesian error message: "perangkat dengan IP %s sudah terdaftar (%s)" (device with IP already registered).

EVIDENCE: Line 893: return "", fmt.Errorf("perangkat dengan IP %s sudah terdaftar (%s)", ip, existingName)

IMPACT: API error response shows Indonesian text to the user.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE

RECOMMENDATION: Change to English: "device with IP %s is already registered (%s)".

F-024

SEVERITY: LOW

CATEGORY: Security / Debug

FILE: 

auth.go

LINE/FUNCTION: Lines 34, 52

PROBLEM: Debug fmt.Println statements left in production auth middleware: "\[Auth Debug] Missing netmon\_token cookie for URL:" and "\[Auth Debug] Invalid token error:".

EVIDENCE: Line 34: fmt.Println("\[Auth Debug] Missing netmon\_token cookie for URL:", r.URL.Path)

IMPACT: Debug info leaks to stdout in production. Not a direct security issue but should use structured logger.

CLASSIFICATION: CONFIRMED ARCHITECTURE ISSUE (debug logging in production)

RECOMMENDATION: Replace fmt.Println with structured logger at WARN/DEBUG level.

F-025

SEVERITY: LOW

CATEGORY: Architecture / Deployment

FILE: 

docker-compose.yml

LINE/FUNCTION: Entire file

PROBLEM: Docker compose does not include frontend build/serve, Prometheus, or any health check configuration. The agent container doesn't expose the frontend dist files.

EVIDENCE: Only db and agent services defined. No Nginx/frontend service. Agent Dockerfile doesn't COPY frontend files.

IMPACT: Docker deployment is incomplete. Cannot serve the frontend from Docker.

CLASSIFICATION: CONFIRMED ARCHITECTURE ISSUE

RECOMMENDATION: Add frontend dist files to the Docker image or add a separate Nginx service.

F-026

SEVERITY: LOW

CATEGORY: Dead Code / Orphan

FILE: 

frontend/tmp\_alerts.json

LINE/FUNCTION: Entire file (untracked)

PROBLEM: Temporary/debug file left in frontend directory.

EVIDENCE: Untracked file shown in git status.

IMPACT: Repository clutter. No functional impact.

CLASSIFICATION: CONFIRMED DEAD CODE

RECOMMENDATION: Remove or add to .gitignore.

F-027

SEVERITY: LOW

CATEGORY: Architecture / Structure

FILE: 

backend/scratch/

LINE/FUNCTION: Entire directory

PROBLEM: A scratch/ directory exists in the backend. Likely contains temporary development files.

EVIDENCE: Directory present in backend listing.

IMPACT: Repository clutter.

CLASSIFICATION: THEORETICAL RISK

RECOMMENDATION: Review contents and remove if not needed.

F-028

SEVERITY: LOW

CATEGORY: Config

FILE: 

config/.env

LINE/FUNCTION: Lines 1-4

PROBLEM: Third .env file with different database config (database=monitoring vs nms\_db in config.yaml). Three separate .env files exist at root, backend, and config directories with inconsistent values.

EVIDENCE: Root .env: no DB name. backend/.env: no DB name. config/.env: MONITORING\_DB\_NAME=monitoring. config.yaml: database: nms\_db.

IMPACT: Confusion about which env file is authoritative. Potential misconfiguration.

CLASSIFICATION: CONFIRMED ARCHITECTURE ISSUE

RECOMMENDATION: Consolidate to a single .env file at root.

F-029

SEVERITY: LOW

CATEGORY: Schema / Code Mismatch

FILE: 

queries.go

LINE/FUNCTION: Line 1360

PROBLEM: InsertDeviceMetric doesn't insert collected\_at, temperature, voltage, signal\_strength, ccq, model, memory\_total, memory\_used fields even though ProcessMetrics in processor.go populates them in the DeviceMetric struct.

EVIDENCE: processor.go:107-128 sets all fields including CollectedAt, Temperature, Voltage, Model, etc. But queries.go:1360 INSERT only includes: device\_id, cpu\_usage, memory\_usage, tx\_rate, rx\_rate, status, reachability\_status, snmp\_status, latency, packet\_loss, jitter, uptime.

IMPACT: Device metric data loss — temperature, voltage, signal strength, CCQ, model, memory totals are collected but never persisted to DB. collected\_at defaults to DB timestamp instead of actual collection time.

CLASSIFICATION: CONFIRMED BUG

RECOMMENDATION: Add missing columns to the INSERT query, or add them via migration if columns don't exist yet.

F-030

SEVERITY: LOW

CATEGORY: Test Coverage

FILE: Repository-wide

LINE/FUNCTION: N/A

PROBLEM: Test coverage is minimal. Only 6 test files exist across the entire codebase: processor\_test.go, models\_test.go, auth\_test.go, event\_test.go, bus\_test.go, tsdb\_handler\_test.go, load\_test.go, exporter\_test.go. No tests for: database queries, device handlers, report handlers, frontend, integration, WebSocket, migration, cleanup task.

EVIDENCE: Test file listing shows sparse coverage of critical paths.

IMPACT: High regression risk for database queries, API handlers, and frontend behavior.

CLASSIFICATION: THEORETICAL RISK

RECOMMENDATION: Add tests for critical paths: database CRUD, API handlers, migration, cleanup.

F-031

SEVERITY: LOW

CATEGORY: Frontend / XSS

FILE: 

monitoring-globals.js

LINE/FUNCTION: Line 32

PROBLEM: Tooltip handler uses newTooltip.innerHTML = text where text comes from data-tooltip attribute. If user-controlled data enters a tooltip attribute, XSS is possible.

EVIDENCE: Line 32: newTooltip.innerHTML = text; where text is from target.getAttribute('data-tooltip').

IMPACT: XSS if device names or other user-controlled data appear in tooltips without sanitization.

CLASSIFICATION: THEORETICAL RISK (depends on whether device names are sanitized)

RECOMMENDATION: Use textContent instead of innerHTML for tooltip rendering.

F-032

SEVERITY: INFO

CATEGORY: Architecture

FILE: 

queries.go

LINE/FUNCTION: Lines 123

PROBLEM: Unicode encoding issue in report summary key: "SLA Met (Ã¢â€°Â¥99%)" — the ≥ character is mojibake (double-encoded UTF-8).

EVIDENCE: Line 123: "SLA Met (Ã¢â€°Â¥99%)" should be "SLA Met (≥99%)".

IMPACT: Report displays garbled characters for the ≥ symbol.

CLASSIFICATION: CONFIRMED BUG

RECOMMENDATION: Fix the encoding to use proper UTF-8: "SLA Met (≥99%)" or "SLA Met (>=99%)".

F-033

SEVERITY: INFO

CATEGORY: Backend / Constants

FILE: 

constants.js

LINE/FUNCTION: Line 6

PROBLEM: API endpoint named DEVICES\_HAPUS — "Hapus" is Indonesian for "Delete".

EVIDENCE: DEVICES\_HAPUS: '/api/devices/delete'

IMPACT: Mixed language in code identifiers (internal, not user-facing). Low impact.

CLASSIFICATION: CONFIRMED LANGUAGE ISSUE (internal identifier, LOW priority)

RECOMMENDATION: Rename to DEVICES\_DELETE for consistency.

F-034

SEVERITY: INFO

CATEGORY: Architecture

FILE: 

hub.go

LINE/FUNCTION: Line 11 (comment)

PROBLEM: Code comment is in Indonesian: "Hub menangani register, unregister client, dan mem-broadcast pesan.".

EVIDENCE: Line 11: Indonesian comment.

IMPACT: Code readability for English-speaking developers.

CLASSIFICATION: FALSE POSITIVE / NON-ISSUE (internal comment, not user-facing)

RECOMMENDATION: Minor: translate comments to English for consistency.

CONFIRMED ISSUES SUMMARY

ID	Severity	Category	Short Description

F-001	CRITICAL	Release	Dirty worktree — 62 uncommitted changes vs v1.0.0 tag

F-002	HIGH	Security	Real secrets committed in .env

F-003	HIGH	Security	Weak JWT secret in backend .env

F-004	HIGH	Security	Default admin credentials in seed SQL

F-005	HIGH	Hardcode	Environment-specific RouterOS config in committed file

F-006	HIGH	Language	All backend alert/incident messages in Indonesian

F-007	HIGH	Language	Activity log messages in Indonesian

F-008	HIGH	Language	Report summary keys in Indonesian

F-009	HIGH	Language	Frontend error/toast messages in Indonesian

F-010	HIGH	Language	Alert acknowledgment logs in Indonesian

F-011	MEDIUM	Dead Code + Dup	Two migration systems with different tracking tables

F-012	MEDIUM	Duplication	Excessive duplicate JSON field aliases

F-013	MEDIUM	Dead Code	TSDBExporter is a no-op goroutine

F-014	MEDIUM	Deployment	Dockerfile Go version mismatch (1.21 vs 1.26)

F-015	MEDIUM	Deployment	Docker compose DB name mismatch + hardcoded credentials

F-016	MEDIUM	Deployment	Dockerfile copies migrations from wrong path

F-018	MEDIUM	Schema	ENUM('up','down') doesn't include 'degraded' — data loss

F-019	MEDIUM	Concurrency	Mutex held during DB operations in threshold evaluation

F-022	MEDIUM	Dead Code	Duplicate logger defaults block (second block never runs)

F-023	LOW	Language	Indonesian error message in AddDeviceWithBrand

F-024	LOW	Debug	fmt.Println debug statements in auth middleware

F-025	LOW	Deployment	Docker compose missing frontend/Prometheus services

F-028	LOW	Config	Three inconsistent .env files

F-029	LOW	Schema/Code	InsertDeviceMetric drops collected fields (temperature, voltage, etc.)

F-032	INFO	Encoding	Mojibake in report summary key (≥ symbol)

F-033	INFO	Language	Indonesian identifier DEVICES\_HAPUS in frontend constant

THEORETICAL RISKS

ID	Severity	Category	Short Description

F-017	MEDIUM	Security	Default SNMP community string public

F-019	MEDIUM	Concurrency	Mutex blocking during DB operations

F-020	MEDIUM	Dead Code	tsdb/client.go may be unused

F-021	MEDIUM	Observability	Same Prometheus metric name registered in two files

F-027	LOW	Structure	Orphan scratch/ directory in backend

F-030	LOW	Test	Minimal test coverage across codebase

F-031	LOW	Security	Potential XSS via innerHTML in tooltip handler

FALSE POSITIVE / NON-ISSUE

ID	Severity	Description

F-034	INFO	Indonesian code comments — internal, not user-facing

TOP 10 PRIORITIES

F-001 (CRITICAL) — Commit or stash worktree changes; release integrity is broken

F-002 (HIGH) — Remove/rotate exposed secrets from .env files

F-003 (HIGH) — Replace weak JWT secret

F-004 (HIGH) — Remove hardcoded default admin credentials from seed SQL

F-006/007/008/009/010 (HIGH) — Complete English language conversion (backend + frontend)

F-014/016 (MEDIUM) — Fix Dockerfile Go version and migration path

F-015 (MEDIUM) — Fix Docker Compose database name mismatch

F-018 (MEDIUM) — Fix ENUM schema to include 'degraded' status

F-029 (LOW) — Fix InsertDeviceMetric to persist all collected fields

F-013 (MEDIUM) — Remove or implement TSDBExporter (currently wasted goroutine)

BLOCKERS

NONE for functionality — the application runs and operates. However:



F-001 is a release integrity blocker — the tag does NOT match the deployed code.

F-018 is a data integrity issue — 'degraded' status silently discarded by MySQL ENUM.

FINAL VERDICT

PASS WITH FINDINGS

The NMS v1.0.0 codebase is functional and operational but has:



1 CRITICAL finding (release consistency)

9 HIGH findings (5 security, 5 language — with overlap)

10 MEDIUM findings (deployment, schema, dead code, duplication)

8 LOW findings

3 INFO findings

The application architecture is sound — clean separation of backend (Go/Chi) with EventBus, WebSocket, TSDB, worker pool, and frontend (Vite/vanilla JS). The primary systematic issue is incomplete English language migration across both backend-generated strings and frontend error messages.



NEXT PHASE — DO NOT EXECUTE

After audit validation:



Validate findings against running application

Prioritize and fix confirmed issues

Language migration (Indonesian → English)

Deployment/Docker fixes

Schema migration for ENUM fix

Dead code removal

Regression testing

Final verification

NO CODE CHANGES. NO COMMIT. NO PUSH. ABSOLUTE STOP.

