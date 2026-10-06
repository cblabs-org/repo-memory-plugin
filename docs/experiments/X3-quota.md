# X3 Quota Measurement

**Date:** 2026-10-05
**Branch:** pr-06-v2-spike / WO5

## Quota API Findings

The command `agy -p /usage --output-format json` provides structured quota information. The returned JSON structure includes:
- A `command.data.groups` array, which contains bucket lists (e.g., "All Models").
- Each bucket provides the quota for a specific model pool (e.g., `gemini-3.8-flash-high`, `gemini-pro-agent`, `claude-sonnet-4-6`).
- Key fields returned in each bucket include:
  - `id`: The internal model bucket ID.
  - `name`: Human-readable name.
  - `remaining_fraction`: A high-resolution float (e.g., `0.7670000195503235`) representing the remaining quota.
  - `reset_time`: An ISO 8601 timestamp (e.g., `2026-10-05T23:22:13Z`) indicating exactly when the window resets.
  - `description`: A human-readable string indicating the window behavior (e.g., "Quota resets in 46 minutes", "Quota resets in 4 hours, 11 minutes").

The `remaining_fraction` provides resolution well beyond 0.1%, and the `reset_time` allows exact calculation of when quota will be replenished.

## Token Measurement Methodology and Results

To measure conversation token costs, we attempted to read the transcripts from the X5 and X8 test conversations, looking for `gen_metadata` fields in `PLANNER_RESPONSE` steps (specifically `input_tokens`, `output_tokens`, and `cached_tokens`).

**Result:** Token metadata (`gen_metadata` or `usage`) was **NOT found** in the `transcript_full.jsonl` files in this environment (as corroborated by the X8 experiment). Token metrics are not exposed in the transcript format.

## Recommendations for Quota Guard Implementation

1. **Use `agy -p /usage --output-format json`:** The quota guard should invoke this command to fetch the current quota state without spending tokens.
2. **Parse the JSON response:** Extract the `command.data.groups[0].buckets` array and find the bucket corresponding to the model the pipeline intends to use.
3. **Threshold Check:** Evaluate the `remaining_fraction` field. If the value falls below a defined safety threshold (e.g., `0.25`), the quota guard can block or defer the pipeline job.
4. **Retry Scheduling:** Use the `reset_time` field to precisely schedule when the sidecar should wake up and retry the deferred jobs, avoiding unnecessary polling.
