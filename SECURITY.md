# Security Policy - DigiPanel Platform

## Supported Versions

DigiPanel issues security updates and patch releases according to the following cadence:

| Version | Supported | Security Patch SLA |
| :--- | :--- | :--- |
| 1.0.x (Current) | Yes | 24 Hours for Critical / 72 Hours for High |
| Development (Master) | Yes | Continuous Integration & Security Auditing |

---

## Reporting a Vulnerability

We prioritize platform security, multi-tenant isolation, and infrastructure hardening.

If you uncover a security vulnerability, do NOT file a public issue. Instead, report it responsibly:

1. **Email**: `security@digihost.id`
2. **PGP Key ID**: `0x7B9A23E1` (Available on keys.openpgp.org)
3. **Information to Include**:
   - Component affected (`agent`, `core`, `modules/*`, `web`)
   - Steps to reproduce or proof-of-concept
   - Potential impact (e.g., privilege escalation, multi-tenant data leakage)

All legitimate security reports receive an acknowledgement within 12 hours and coordinated vulnerability disclosure (CVD) timelines.
