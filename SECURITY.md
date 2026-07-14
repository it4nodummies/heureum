# Security Policy

## Supported versions

Heureum is currently pre-1.1, with the 1.x line as the actively supported release series.
Security fixes are made against the latest `1.x` release. Older pre-release/development
snapshots are not supported.

| Version | Supported          |
|---------|---------------------|
| 1.x     | :white_check_mark: |
| < 1.0   | :x:                 |

## Authorization model

As of 1.0, Heureum **enforces authorization server-side on both mutations and reads**:

- **Mutations** (create/update/delete) are checked against the caller's role on the
  relevant project (admin/member/viewer) or their global-admin flag, returning **403**
  when the caller lacks the required permission. Group and project-category mutations
  require a global admin; filters and dashboards can only be mutated by their owner (or a
  global admin); the creator of a project automatically becomes its admin.
- **Reads** are membership-gated: project-scoped resources (projects, issues, boards,
  sprints, reports, comments, attachments, …) return **404** to non-members — without
  revealing whether the resource exists. Lists and JQL search results are filtered to the
  caller's projects. User email addresses are visible only to the user themselves and to
  global admins. Filters and dashboards are readable by their owner, or when shared/public.
  The board websocket endpoint requires authentication. Global admins see everything.

### Notes for deployers

Other things to be aware of when evaluating deployment risk:

- Issue attachments are implemented (upload/download/delete), but are stored on local disk
  under `APP_UPLOADS_DIR` — mount a persistent volume (Docker) or PVC (the Helm chart does
  this by default) so uploads survive restarts. Object-storage (S3-compatible) backing is
  a planned enhancement.
- SMTP and OAuth integrations are not yet wired up (see the Configuration section of the
  [README](README.md)).

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues, discussions,
or pull requests.**

Instead, report vulnerabilities privately by emailing:

**ivano.turi@harpaitalia.it**

Please include as much of the following as you can:

- A description of the vulnerability and its potential impact
- Steps to reproduce, or a proof-of-concept if available
- The affected version/commit
- Any suggested mitigation, if you have one

### What to expect

- **Acknowledgement**: within 3 business days of your report.
- **Initial assessment**: within 7 business days, including a rough severity estimate and
  next steps.
- **Resolution timeline**: communicated once triage is complete; timing depends on
  severity and complexity, but we aim to ship fixes for critical issues as quickly as
  possible.

We will keep you informed as the issue is triaged and fixed, and will credit you in the
release notes/CHANGELOG unless you prefer to remain anonymous.

## Disclosure policy

We ask that you give us a reasonable opportunity to investigate and address a
vulnerability before any public disclosure. Coordinated disclosure timelines can be
discussed on a case-by-case basis via the contact email above.
