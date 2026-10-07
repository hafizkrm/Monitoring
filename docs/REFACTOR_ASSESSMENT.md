# FULL REFACTOR ASSESSMENT

## P0 (Must Refactor - Critical Technical Debt)
- **`backend/internal/snmp/metrics.go`**: Massive monolith (57KB, 1760 lines). `CollectDeviceMetrics` mixes pure SNMP collection with vendor-specific hacks (Ubiquiti, Mikrotik), ICMP fallbacks, and concurrency control. Severe violation of Single Responsibility Principle.
- **`backend/internal/database/queries.go`**: Monolithic repository file (56KB, 1661 lines) housing raw SQL for all domains (devices, telemetry, alerts). DB/query inefficiency due to massive string concatenation and lack of domain-driven repository separation.
- **`frontend/src/core/monitoring-globals.js`**: Gigantic global dumping ground (53KB). Mixes DOM manipulation, tooltip hacks, global state (`window.*`), and complex alert processing logic. Severe frontend/backend coupling and maintainability risk.

## P1 (High-Value - Architectural Simplification)
- **`frontend/src/modules/devices/devices.js` & `deviceDetail.js`**: Huge scripts (45KB & 34KB) performing manual DOM manipulation, fetching, and state management in a single flow. Needs componentization or Model-View separation.
- **`frontend/src/modules/dashboard/dashboard.html` & `dashboard.js`**: Massive view file (45KB HTML) with duplicated layouts, hardcoded elements, and ad-hoc scripts. 
- **`frontend/src/modules/reports/reports.js` & `logs/logs.js`**: Large files (41KB each) mixing data formatting, API calls, and complex DOM rendering.
- **Global Event Handlers**: Too many inline `onclick` bindings relying on globally attached `window.*` functions. 

## P2 (Optional - Code Polish)
- **`backend/internal/worker/manager.go` & `processor.go`**: Polling manager logic is robust but somewhat complex with circuit breakers, retry logic, and cache management intertwined. Could be simplified using cleaner worker pool abstractions.
- **`frontend/index.html` & `login.html`**: Large monolithic HTML files (25KB+). Could be refactored into a templating system or standard layout includes.

## KEEP (Appropriate Architecture)
- **Backend Directory Structure**: The `cmd/agent`, `internal/api`, `internal/worker`, and `internal/snmp` separation is idiomatic Go and provides a good architectural boundary.
- **Frontend Modular Structure**: The physical folder separation under `frontend/src/modules/` is conceptually good, even if the files inside are too large.
- **Go Interfaces**: The usage of interfaces in `contracts/` and `interfaces.go` for `DatabaseClient` and `CollectorRegistry` is appropriate for testing.

## TOP 10 TARGETS
1. `backend/internal/snmp/metrics.go` (Split by metric type and vendor)
2. `backend/internal/database/queries.go` (Split into specific repository structs)
3. `frontend/src/core/monitoring-globals.js` (Modularize into utilities, state, and UI modules)
4. `frontend/src/modules/devices/devices.js` (Extract rendering logic)
5. `frontend/src/modules/dashboard/dashboard.html` (Componentize UI)
6. `frontend/src/modules/reports/reports.js` (Separate data fetch from DOM logic)
7. `frontend/src/modules/logs/logs.js` (Separate data fetch from DOM logic)
8. `frontend/src/modules/devices/deviceDetail.js`
9. `backend/internal/worker/manager.go` (Extract circuit breaker logic)
10. `frontend/index.html` (Layout abstraction)

## DEPENDENCIES
- **Backend**: `gosnmp` (Core monitoring engine), `database/sql` (direct DB access).
- **Frontend**: Vanilla JS with Vite build system, Chart.js for visualization, FontAwesome.

## RISKS
- **Vendor-Specific Regressions**: Splitting `metrics.go` carries high risk of breaking nuanced vendor-specific (Ubiquiti/Mikrotik) fallback logic for uptime and reachability.
- **Global State Breakage**: Dismantling `monitoring-globals.js` will likely break inline `onclick` handlers across multiple HTML files unless an event delegation or adapter pattern is used.
- **Query Breakage**: Refactoring `queries.go` into domain repositories risks misaligning SQL arguments and struct fields, requiring extensive integration testing.
