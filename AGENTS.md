# URock Agent Guide

All implementation work must begin by reading `PRD.md`.

## Scope

- Implement only the active PRD milestone or explicitly requested vertical slice.
- Ask before changing rules involving money, card validity, permissions, refunds, redemption, or deletion.
- Keep the backend as a modular monolith. Do not add Redis, queues, microservices, Kubernetes, or object storage without an explicit requirement.

## Required checks

Run `make check` before handing off changes. Payment, refund, and redemption work also requires MySQL-backed integration tests for transaction and uniqueness behavior.

## Security

- Never commit secrets or production credentials.
- Never log tokens, WeChat session keys, payment keys, complete phone numbers, or redemption token plaintext.
- Frontend visibility is not authorization; enforce RBAC and ownership in the Go API.

## Repository map

- `backend/`: Go API and SQL migrations.
- `miniprogram/`: native WeChat mini program written in TypeScript.
- `deploy/`: deployment configuration.
- `PRD.md`: product and engineering contract.
- `backend/api/openapi.yaml`: API contract; update it with endpoint changes.

