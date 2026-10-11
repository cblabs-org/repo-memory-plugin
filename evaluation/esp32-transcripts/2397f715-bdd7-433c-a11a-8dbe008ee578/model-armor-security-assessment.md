# Model Armor Security & Auth Assessment

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  Host Machine (your Mac)                                        │
│                                                                 │
│  ┌─────────────┐    ┌──────────────┐    ┌────────────────────┐ │
│  │ Triage      │───▶│ Injection    │───▶│ Model Armor API    │ │
│  │ Dispatcher  │    │ Scanner      │    │ (europe-west2)     │ │
│  └─────────────┘    └──────┬───────┘    └─────────┬──────────┘ │
│                            │                      │             │
│                     ┌──────▼───────┐    ┌─────────▼──────────┐ │
│                     │ Injection    │    │ GCP Project:       │ │
│                     │ Cache        │    │ bug-fixer-model-   │ │
│                     │ (local JSON) │    │ armor              │ │
│                     └──────────────┘    └────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Auth Chain

```
Your gcloud identity (matthew.mcneill@copperbeech.me.uk)
    │
    │  has: roles/iam.serviceAccountTokenCreator
    ▼
Service Account (model-armor-scanner@bug-fixer-model-armor.iam.gserviceaccount.com)
    │
    │  has: roles/modelarmor.user (on project bug-fixer-model-armor)
    ▼
Model Armor API (sanitizeUserPrompt)
```

**How it works:**
1. `gcloud auth print-access-token --impersonate-service-account=...` mints a short-lived OAuth2 token (3600s)
2. Token is cached in-memory with 60s buffer before re-mint
3. Token sent as `Authorization: Bearer` header to the regional API endpoint

## Security Controls Assessment

### ✅ What's Done Well

| Control | Implementation | Rating |
|---------|---------------|--------|
| **A1: Dedicated project** | `bug-fixer-model-armor` — isolated from other workloads | ✅ Strong |
| **A2: Scoped SA** | `model-armor-scanner` has ONLY `roles/modelarmor.user` — can call sanitizeUserPrompt and nothing else | ✅ Strong |
| **A3: No JSON keys** | Uses `--impersonate-service-account` — no key files on disk. Tokens are short-lived (1hr), in-memory only | ✅ Strong |
| **A4: Quota cap** | Not explicitly set, but the dedicated project means billing is isolated | 🟡 Partial |
| **Fail-closed on 429** | Rate-limit errors (after 5 retries with exponential backoff) always return `is_injection=True` — an attacker can't bypass by flooding the API | ✅ Strong |
| **Fail-open configurable** | `fail_open: true` means non-429 errors let text through. This is the right default for availability — a GCP outage shouldn't block the entire bug-fixer loop | ✅ Reasonable |
| **Config protection** | `config.json` (which holds the project/SA) is protected by Gate A (filename regex), Gate A relative paths, AND the integrity monitor (PostToolUse checksum verification + git revert) | ✅ Strong |
| **Cache poisoning prevention** | Only successful provider verdicts are cached. Disabled/error results are never cached | ✅ Strong |
| **Cache key scoping** | Key includes `provider:template:text` hash — prevents cross-config cache hits | ✅ Strong |
| **Execution state validation** | If Model Armor returns anything other than `EXECUTION_SUCCESS`, raises RuntimeError (no silent failures) | ✅ Strong |

### 🟡 Areas for Improvement

| Area | Current State | Risk | Recommendation |
|------|--------------|------|----------------|
| **A4: Quota cap** | No explicit per-minute/per-day quota set on the GCP project | Low — attacker would need to compromise the host to abuse | Set `modelarmor.googleapis.com` quota in GCP Console to e.g. 100 req/min |
| **Token exposure in process list** | `gcloud auth print-access-token --impersonate-service-account=...` token appears briefly in stdout. In-memory cache means this only happens once per hour | Low — requires local process snooping | Acceptable given A5 is deferred |
| **fail_open: true** | A sustained GCP outage would let injection attacks through unscanned | Medium — but the alternative (fail_closed) would halt the entire loop | Consider monitoring: log `INJECTION_SCAN_ERROR` events and alert if sustained |
| **No mutual TLS** | Standard HTTPS to `modelarmor.europe-west2.rep.googleapis.com` | Very low — GCP endpoints use Google-managed TLS certificates | Standard practice |

### 🔴 Known Accepted Limitations

| Limitation | Status | Tracked |
|-----------|--------|---------|
| **A5: Separate OS user** | Deferred — dispatchers run as the same OS user. An agent with shell access could theoretically read the cached token from `/proc` or intercept the `gcloud` call | #22 |
| **Skill migration** | Skills/agents still use raw `gh` — only dispatchers go through the egress proxy | #22 |
| **python3 -c bypass** | Regex gate can be evaded, but the integrity monitor now catches and reverts protected file modifications | Mitigated |

## Data Flow Security

```
Issue text (title + body)
    │
    ▼
CachedInjectionScanner.scan()
    │
    ├── Cache HIT? → return cached verdict (no API call)
    │
    ├── Cache MISS → ModelArmorProvider.scan()
    │       │
    │       ├── Mint token via gcloud impersonation
    │       │       └── SA: model-armor-scanner@...
    │       │           └── Role: roles/modelarmor.user ONLY
    │       │
    │       ├── POST to sanitizeUserPrompt
    │       │       └── Payload: {"userPromptData": {"text": "..."}}
    │       │       └── Only sends the text, no repo/user metadata
    │       │
    │       ├── 429? → retry with exponential backoff (up to 5×)
    │       │       └── All retries exhausted? → fail CLOSED
    │       │
    │       └── Parse response → InjectionResult
    │
    └── If injection detected:
            ├── Label issue: security:prompt-injection
            └── Exclude from triage agent (quarantine)
```

> [!IMPORTANT]
> **No issue text is stored in GCP.** The Model Armor API processes text
> ephemerally — Google does not retain sanitizeUserPrompt payloads.
> The only persistent record is in the local `injection_cache.json` (SHA-256
> hash → verdict, no raw text stored).

## Overall Rating

**Strong for the threat model.** The primary threat is a compromised or manipulated AI agent attempting to:
1. **Bypass scanning** — mitigated by fail-closed on rate limits, config protection (3 layers), and cache poisoning prevention
2. **Abuse the GCP credential** — mitigated by dedicated project, scoped SA (can only call sanitizeUserPrompt), no key files, short-lived tokens
3. **Tamper with config** — mitigated by Gate A regex, relative path protection, and PostToolUse integrity monitor with auto-revert
4. **Poison the cache** — mitigated by only caching successful verdicts, provider-scoped keys

The main residual risk is **A5 (separate OS user)** — without it, a determined agent with shell access could theoretically intercept the gcloud token. This is tracked on #22 as a future hardening step.
