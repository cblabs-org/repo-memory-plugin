package claims

import (
	"testing"
)

func TestParseWrite(t *testing.T) {
	data := []byte(`---
id: clm-01J8ZQ4K2V7X
type: invariant
title: BLE host and controller memory must be internal DRAM
relevant_when: changing BLE initialisation, NimBLE memory settings or PSRAM allocation
apply: Keep CONFIG_BT_NIMBLE_MEM_ALLOC_MODE_EXTERNAL=n and allocate no BLE buffers in PSRAM
status: verified
flags: []
anchors:
  - kind: config
    ref: CONFIG_BT_NIMBLE_MEM_ALLOC_MODE_EXTERNAL
    fp: val:n
  - kind: symbol
    ref: BLEStack::init
    fp: sha1:9f2c
  - kind: subsystem
    ref: ble
conditions:
  chip: esp32s3
  esp-idf: 5.5.5
  esp-nimble-cpp: 2.5.0
evidence:
  - kind: episode
    ref: ep-01J8X
    steps:
      - 112
      - 140
    outcome: reset:INT_WDT
  - kind: episode
    ref: ep-01J8Y
    steps:
      - 31
    outcome: soak_pass:15m
  - kind: commit
    ref: 4e1a9c2
    outcome: merged
relations:
  explains:
    - clm-01J8ZQ4K2V7Y
scope:
  branch: main
verified_at: 69d9c813
provenance:
  pipeline: 1.0.0
  extractor_model: small
  reconciler_model: large
review:
  state: approved
  by: "@matthewmcneill"
  at: "2026-09-24"
created: 2026-09-23T22:10:00Z
updated: 2026-09-24T09:02:00Z
---

BLE controller DMA cannot access PSRAM. With MEM_ALLOC_MODE_EXTERNAL=y,
NimBLEDevice::init() holds cross-core heap spinlocks long enough to trigger
INT_WDT/TG0WDT resets on ESP32-S3. All BLE memory must be internal DRAM.`)

	claim, err := Parse(data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claim.ID != "clm-01J8ZQ4K2V7X" {
		t.Errorf("expected ID clm-01J8ZQ4K2V7X, got %s", claim.ID)
	}
	if claim.Type != TypeInvariant {
		t.Errorf("expected invariant, got %s", claim.Type)
	}
	if claim.Title != "BLE host and controller memory must be internal DRAM" {
		t.Errorf("unexpected title: %s", claim.Title)
	}
	if len(claim.Anchors) != 3 {
		t.Errorf("expected 3 anchors, got %d", len(claim.Anchors))
	}
	if len(claim.Evidence) != 3 {
		t.Errorf("expected 3 evidence items, got %d", len(claim.Evidence))
	}

	written, err := Write(claim)
	if err != nil {
		t.Fatalf("expected no error on write, got %v", err)
	}

	// Just a sanity check that it parses back
	claim2, err := Parse(written)
	if err != nil {
		t.Fatalf("expected written content to be parseable, got %v", err)
	}

	if claim2.ID != claim.ID {
		t.Errorf("ID mismatch after write/parse cycle")
	}
}

func TestClosesWhenUnknown(t *testing.T) {
	data := []byte(`---
id: clm-01J8ZQ4K2V7Z
type: open-issue
title: Known defect
status: verified
flags: []
closes_when: unknown
created: 2026-09-23T22:10:00Z
updated: 2026-09-24T09:02:00Z
---

Test body`)

	claim, err := Parse(data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claim.ClosesWhen == nil || !claim.ClosesWhen.Unknown {
		t.Errorf("expected ClosesWhen.Unknown to be true")
	}

	written, err := Write(claim)
	if err != nil {
		t.Fatalf("expected no error on write, got %v", err)
	}

	claim2, err := Parse(written)
	if err != nil {
		t.Fatalf("expected written content to be parseable, got %v", err)
	}

	if claim2.ClosesWhen == nil || !claim2.ClosesWhen.Unknown {
		t.Errorf("expected ClosesWhen.Unknown to be true after write/parse")
	}
}
