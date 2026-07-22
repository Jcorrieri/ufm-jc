# UF Marketplace Agent Guide

## Purpose

UF Marketplace is a student marketplace demo. Refactoring should make the code easier to
understand and maintain, establish reliable automated testing and CI/CD, and move image bytes
from SQLite to object storage.

## Current Structure

- `frontend/`: Angular 21 single-page application. Views and reusable components call Angular
  services for REST and WebSocket APIs. Vitest unit specs and Cypress browser specs exist.
- `backend/`: Go/Gin API organized into handlers, services, GORM models, middleware, database
  setup, and utilities. Tests currently focus mainly on services, models, JWTs, and middleware.
- `.github/workflows/`: Read-only GitHub Actions CI with parallel backend and frontend jobs for
  pushes and pull requests targeting `main` or `dev`.
- `backend/app/`: HTTP application composition and route registration.
- `backend/cmd/`: Separate server, migration, and seed entrypoints.
- `backend/config/`: Centralized environment configuration loading and validation.
- `backend/database/`: Explicit SQLite connection, schema migration, and optional seed operations.
- `Sprint*.md`: Historical course planning notes; treat these as context, not current
requirements.

## Architecture and Data Flow

The Angular client sends requests under `/api`. Gin handlers parse HTTP input and authorization
context, services perform business logic and GORM access, and models define persistence and API
response shapes. Authentication uses a signed JWT in an HttpOnly cookie. Chat uses an in-memory
WebSocket hub while persisting messages in SQLite. Uploaded JPEG/PNG files are validated in Go,
stored as image BLOB rows, and served through `/api/images/:imageId`.

## Refactoring Goals

1. Clarify boundaries: keep transport concerns in handlers, business rules in services, and
   persistence behind small interfaces. Centralize typed configuration and application startup.
2. Improve tests: add handler/API integration coverage, isolate databases per test, and expand
   frontend tests around critical user flows. Keep tests deterministic and independent of
   external image downloads.
3. Maintain CI checks for Go formatting, vet, tests, and builds plus Angular tests and builds.
   Add Cypress only after its server orchestration and fixtures are reliable in CI.
4. Extract image storage: define an object-storage interface, store only object keys and metadata
   in the database, validate upload size and decoded image type, and support deletion/rollback.
   Use an S3-compatible local implementation for development and tests before migrating data.

## Working Conventions

- Preserve existing API behavior unless a change is intentional and covered by tests.
- Prefer dependency-free, modular changes; discuss new dependencies before adding them.
- Keep lines under 100 characters and favor readable names over abbreviations.
- From `backend/`, run `go test ./...`, `go vet ./...`, and `go build ./...` for affected work.
- From `frontend/`, run `npm test -- --watch=false` and `npm run build` for affected work.
- Run Cypress locally with `npm start` and `npm run cypress:run` when browser flows change.
- Never commit `.env`, database files, credentials, generated images, or object-store data.
- Update this document when architecture, commands, or known risks materially change.

## Frontend Test Coverage

- Angular/Vitest has 39 tests in nine specs. Forgot/reset password and order history have
  behavioral coverage; app, avatar, listing, navbar, login, and sign-up specs are creation-only
  smoke tests.
- Cypress exercises 89 browser scenarios with intercepted APIs across login, registration,
  password reset, marketplace search, listing CRUD and images, auth guards, and order history.
- Cypress runs against Angular at `http://localhost:4200`. Because most API calls are intercepted,
  it validates browser, component, and routing behavior rather than the deployed full stack.
- Main search, product details, profile, settings, messaging, services, and guards have limited or
  no focused Angular unit coverage. Keep tests aligned when routes, selectors, or API calls change.

## Limitations, Security Issues, and Architectural Debt

- SQLite remains the only database adapter, although connection, migration, and seeding lifecycles
are explicit and separate from HTTP server startup.
- Image BLOBs inflate the transactional database and API process memory. Uploads are read fully
into memory, images are publicly addressable by ID, and lifecycle cleanup is incomplete.
- WebSockets accept every origin. Restrict origins and retain participant authorization checks.
- Auth cookies are set with `Secure=false`; production needs HTTPS-only cookies, explicit cookie
policy, CSRF protection, and validated non-empty secrets at startup. Logout does not revoke JWTs.
- The forgot-password response exposes the reset token when an account exists. Gate that behavior
behind an explicit development mode and use out-of-band delivery in deployed environments.
- Public auth and password-reset endpoints have no visible throttling. Password policy is only a
six-character minimum, and registration checks the email suffix without verifying ownership.
- Listing/conversation inputs do not consistently enforce ownership and relational integrity;
for example, a client supplies the seller ID when starting a conversation.
- The in-memory chat hub supports only one API process and loses active connections on restart.
- Several error paths continue after writing a response, configuration errors may panic,
  and logging is unstructured.
- Cypress is not run in CI and relies on mocked APIs plus a separately started Angular server.
