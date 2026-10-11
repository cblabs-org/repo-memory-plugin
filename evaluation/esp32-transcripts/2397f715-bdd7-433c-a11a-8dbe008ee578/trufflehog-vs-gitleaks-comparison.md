# Trufflehog vs Gitleaks — Head-to-Head Comparison

**Context:** Selecting the default secret scanning backend for [Issue #22](https://github.com/matthewmcneill/bug-fixer-loop-plugin/issues/22) WP2/WP3.
**Versions compared:** Trufflehog v3.97.9, Gitleaks v8.30.1 (both as of October 2026)

---

## The Decision Table

| Dimension | Gitleaks | Trufflehog | Winner for this project |
|-----------|----------|------------|------------------------|
| **Detection rules** | 222 rules | 904 detectors | 🔶 Trufflehog (4x more) |
| **Network calls by default** | **None** — zero network I/O | **Yes** — verification + auto-update | ✅ **Gitleaks** |
| **Output redaction** | Built-in `--redact` flag | No CLI redaction flag; `Raw` always in JSON | ✅ **Gitleaks** |
| **Staged diff scanning** | Native `git diff --staged`; sub-50ms | Clones repo to `/tmp`; copies git index | ✅ **Gitleaks** |
| **Binary size** | ~20 MB (~8 MB compressed) | ~80-120 MB (~35-78 MB compressed) | ✅ **Gitleaks** |
| **Performance** | 50-200ms for staged diffs; <50 MB RAM | Minutes with verification; 200-500 MB+ RAM | ✅ **Gitleaks** |
| **False positive suppression** | `.gitleaksignore` + `#gitleaks:allow` + baselines | `trufflehog:ignore` inline + path exclusion only | ✅ **Gitleaks** |
| **License** | **MIT** | **AGPL-3.0** | ✅ **Gitleaks** |
| **CVEs** | **Zero** | CVE-2024-43379 (Blind SSRF), CVE-2025-41390 (Git ACE) | ✅ **Gitleaks** |
| **Maintenance** | Feature-complete; security patches only | Active weekly releases | 🔶 Trufflehog |
| **Entropy analysis** | Native per-rule Shannon entropy | Deprecated in v3; optional post-filter only | ✅ **Gitleaks** |
| **Verification (is key live?)** | None | Active API calls to providers | 🔶 Trufflehog (opt-in only) |

---

## The Three Things That Matter Most

### 1. Network Behaviour — Gitleaks is safe by default, Trufflehog is not

Trufflehog makes **two kinds** of unsolicited network calls:

- **Verification:** When it finds a candidate secret, it tries to *use it* against the real service (AWS `sts:GetCallerIdentity`, GitHub `/user`, Slack `auth.test`, etc.) to check if the key is live.
- **Auto-update:** Uses `github.com/jpillora/overseer` to check for new versions against GitHub Releases on every execution.

Both must be explicitly disabled (`--no-verification --no-update`). If either flag is forgotten, the autonomous agent is making authenticated calls to third-party services and pulling remote binaries — with no human in the loop.

Gitleaks makes **zero** network requests. No verification, no telemetry, no auto-updater. There is nothing to disable.

### 2. Security Track Record — Trufflehog's verification has been actively exploited

| CVE | Impact | Root cause |
|-----|--------|-----------|
| **CVE-2024-43379** | Blind SSRF — crafted repo data triggers detectors to make requests to attacker-controlled endpoints | Verification logic |
| **CVE-2025-41390** | Arbitrary code execution via crafted git repositories | Git scanning routines |

Beyond CVEs, Trufflehog's verification also:
- **Detonates canary tokens and honeypots** — scanning a repo containing Canarytokens will trigger alerts and expose the scanner's IP
- **Alerts adversaries** — verifying a stolen token creates an audit log entry at the provider, potentially tipping off an attacker
- **Triggers rate limiting** — scanning large histories can trip API rate limits at AWS, GitHub, Slack, etc.

Gitleaks has **zero** published CVEs and zero GitHub Security Advisories. As a purely offline tool, it is architecturally invulnerable to SSRF, credential exfiltration, and canary detonation.

### 3. The Raw Match Problem

Both tools include raw secret text in their JSON output by default. But:

- **Gitleaks** has a built-in `--redact` flag that replaces secrets in output with `REDACTED`
- **Trufflehog** always includes the full secret in `Raw` and `RawV2` JSON fields — **there is no CLI flag to suppress this**

This directly relates to our WP0a bug: `scan_repository()` currently leaks matched secrets into agent context. With Gitleaks, the wrapper just passes `--redact`. With Trufflehog, the wrapper must parse the JSON and strip `Raw`/`RawV2` fields itself — one more place to get wrong.

---

## Secondary Considerations

### Detection Depth

Trufflehog has 904 detectors vs Gitleaks' 222 rules. This sounds like a 4x advantage, but:

- Many of Trufflehog's detectors are *verification-specific* — they detect the same pattern but add verification logic for different services. With verification off, the pattern coverage gap narrows.
- Gitleaks' per-rule entropy thresholds catch high-entropy strings that don't match any specific pattern — a different detection strategy that compensates for fewer named rules.
- Gitleaks rules are user-extensible via TOML — adding a custom rule is a 5-line config change, not a Go code contribution.

### License

Trufflehog is **AGPL-3.0** — any network service that modifies or embeds it must disclose source code. This is a strong copyleft obligation. Gitleaks is **MIT** — fully permissive. For a plugin that may be embedded in commercial projects, MIT is significantly less risky.

### Maintenance Status

Gitleaks is declared **"feature complete"** by its author, with future development shifting to a successor project called **Betterleaks**. This means:
- ✅ Stable, no breaking changes
- ⚠️ New detector rules will slow down
- ⚠️ If a critical bug is found, the response may be slower

Trufflehog ships weekly releases with new detectors and bug fixes, backed by Truffle Security Co. (venture-funded).

### Staged Diff Performance

For `--git-hook` pre-commit use:
- **Gitleaks**: Native `git diff --staged` streaming → **50-200ms**
- **Trufflehog**: Clones repo to `/tmp`, copies git index, scans staged chunks → **seconds to minutes**

In a pre-commit hook, the developer is waiting. Sub-200ms vs multi-second is the difference between "invisible" and "annoying".

---

## Recommendation

**Gitleaks as the default backend.** The decision factors that matter most for this project:

| Constraint | Gitleaks | Trufflehog |
|-----------|----------|------------|
| Autonomous agent with no human in the loop | ✅ Zero network calls | ❌ Must disable verification + auto-update |
| `scan_repository` must not leak secrets (WP0a) | ✅ Built-in `--redact` | ❌ No redaction flag; wrapper must strip `Raw` |
| Pre-commit hook latency | ✅ 50-200ms | ❌ Seconds to minutes |
| Zero CVE history | ✅ | ❌ 2 CVEs (SSRF + ACE) |
| MIT-compatible licensing | ✅ MIT | ❌ AGPL-3.0 |
| Feature-complete stability | ✅ No breaking changes | ⚠️ Weekly releases, occasional breaking |

**Keep Trufflehog as a supported secondary backend** behind the `ScanBackend` interface for users who want the deeper detector coverage — with `--no-verification --no-update` hard-coded, and `verify: true` as an explicit opt-in.

> [!NOTE]
> **Watch Betterleaks.** Gitleaks' author is building a successor. When it matures, it may become the natural upgrade path. The `ScanBackend` abstraction ensures swapping is trivial.
