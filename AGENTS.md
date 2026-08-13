# URock Agent Guide

All implementation work must begin by reading `PRD.md`.

## Scope

- Implement only the active PRD milestone or explicitly requested vertical slice.
- Ask before changing rules involving money, card validity, permissions, refunds, redemption, or deletion.
- Keep the backend as a modular monolith. Do not add Redis, queues, microservices, Kubernetes, or object storage without an explicit requirement.

## Required checks

Run `make check` before handing off changes. Payment, refund, and redemption work also requires MySQL-backed integration tests for transaction and uniqueness behavior.

## Mini program publishing

- Every uploaded version must be published by the WeChat DevTools account named `Jack`.
- Never upload with `miniprogram-ci`; it is reserved for previews and publishes under `CI机器人1`.
- Use `cd miniprogram && npm run upload -- --version=x.y.z --desc="Jack: description"` and confirm the DevTools account is `Jack` first.

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
