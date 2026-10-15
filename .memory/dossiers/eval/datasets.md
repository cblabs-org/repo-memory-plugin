# eval/datasets

## Decisions

### [seed-ef44eeac] Wrote `sample_b5.go` as a standalone script in `internal/eval/scripts` that directly hits the `episode.Store` and writes out a CSV file. Note: The `episode.Header` struct did not have a `Summary` field as originally expected, so I omitted it from the CSV output.
`decision · kind: episodic · anchors: internal/eval/datasets/b1_briefing.json, internal/eval/datasets/b2_questions.json, internal/eval/datasets/b3_replay.json, internal/eval/scripts/sample_b5.go`

### [seed-fbe1b533] Chose to write the datasets in JSON to comply with `eval.LoadDataset` format shown in WO5.
`decision · kind: episodic · anchors: internal/eval/datasets/b1_briefing.json, internal/eval/datasets/b2_questions.json, internal/eval/datasets/b3_replay.json, internal/eval/scripts/sample_b5.go`
