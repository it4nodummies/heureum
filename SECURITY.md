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

As of 1.0, Heureum **enforces authorization server-side on mutating requests**: every
create/update/delete is checked against the caller's role on the relevant project
(admin/member/viewer) or their global-admin flag, returning **403** when the caller lacks
the required permission. Group and project-category mutations require a global admin;
filters and dashboards can only be mutated by their owner (or a global admin); the creator
of a project automatically becomes its admin.

### Known limitation — please read before deploying

**Reads are not yet permission-gated: any authenticated user can read any project's data
(issues, comments, boards, reports).** Read-side authorization (`BROWSE_PROJECTS`) is a
planned enhancement. If read confidentiality between authenticated users matters for your
deployment, restrict access at the network layer until read enforcement ships.

Other things to be aware of when evaluating deployment risk:

- Attachments are not yet implemented.
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
