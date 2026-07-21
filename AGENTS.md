# UF Marketplace Agent Guide

## Purpose

UF Marketplace is a student marketplace demo. Refactoring should make the code easier to
understand and maintain, establish reliable automated testing and CI/CD, and move image bytes
from SQLite to object storage.

## Current Structure

- `frontend/`: Angular 21 single-page application. Views and reusable components call Angular
  services for REST and WebSocket APIs. Vitest unit specs and Cypress end-to-end specs exist.
- `backend/`: Go/Gin API organized into handlers, services, GORM models, middleware, database
  setup, and utilities. Tests currently focus mainly on services, models, JWTs, and middleware.
- `backend/main.go`: Composition root, route registration, configuration lookup, database
  migration, and server startup.
- `backend/database/`: SQLite connection, automatic schema migration, and optional seed data.
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
3. Add CI/CD: run Go formatting, vet/static checks, and tests; run npm clean install, frontend
   tests, and production build; then add Cypress against an ephemeral full stack. Require these
   checks before deployment and keep secrets in the CI provider.
4. Extract image storage: define an object-storage interface, store only object keys and metadata
   in the database, validate upload size and decoded image type, and support deletion/rollback.
   Use an S3-compatible local implementation for development and tests before migrating data.

## Working Conventions

- Preserve existing API behavior unless a change is intentional and covered by tests.
- Prefer dependency-free, modular changes; discuss new dependencies before adding them.
- Keep lines under 100 characters and favor readable names over abbreviations.
- Run `go test ./...` from `backend/` and frontend tests/build from `frontend/` for affected work.
- Never commit `.env`, database files, credentials, generated images, or object-store data.
- Update this document when architecture, commands, or known risks materially change.

## Limitations, Security Issues, and Architectural Debt

- There is no checked-in CI/CD workflow, deployment configuration, or root-level test command.
- SQLite, automatic migrations, seed downloads, and server construction occur during startup;
  this couples infrastructure to the application and limits concurrency and deployment options.
- Image BLOBs inflate the transactional database and API process memory. Uploads are read fully
  into memory, images are publicly addressable by ID, and lifecycle cleanup is incomplete.
- `Search` interpolates a client-controlled column name into SQL. Allow-list searchable fields
  before treating this endpoint as safe.
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
- Several error paths continue after writing a response, configuration errors may panic, the
  server address is hard-coded to localhost, and logging is unstructured.
- The READMEs are stale in places and make unsafe claims that local SQLite eliminates exposure.
  Tests exist but coverage, fixtures, commands, and required quality gates are undocumented.
