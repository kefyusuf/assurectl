# AssureCTL Foundation Design — Amendment 1

**Status:** Accepted  
**Date:** 2026-09-02  
**Amends:** `2026-09-02-assurectl-foundation-design.md`

## Reason

Implementation review identified a self-rooted trust ambiguity in the original contract example: an input-controlled document must not establish its own authority by carrying an `authority` field.

## Amendment

The following rules supersede conflicting examples or wording in the original foundation design:

1. Policy, verification-contract, and evidence payloads do not declare their own trust or authority tier.
2. Input source identity and trust are resolved externally by the loader or integration.
3. Local workspace inputs are advisory because of their source context, not because they self-label as advisory.
4. Authoritative sources require externally protected context such as a trusted base revision, protected control plane, pinned issuer, or source-pinned GitHub App.
5. Receipts record the verifier-resolved source, digest, and trust status for policy and contract inputs.
6. Overall receipt authority remains separate from input trust.
7. Receipt evidence references identify actual supplied evidence. Per-requirement evidence state, observed outcome, waivability, and waiver status are recorded separately so missing evidence does not require fabricated evidence metadata.

ADR-0007 is the normative decision record for this amendment.
