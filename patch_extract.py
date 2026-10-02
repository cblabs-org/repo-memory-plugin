with open("internal/core/pipeline/extract/extractor.go", "r") as f:
    code = f.read()

import re

new_extract = """
func (e *Extractor) Extract(ctx context.Context, episodes []*episode.Episode, triageResults map[string]triage.TriageResult) ([]ExtractionResult, error) {
	packer := NewPacker(e.TokenCap)
	batches := packer.Pack(episodes, triageResults)
	
	var allResults []ExtractionResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, batch := range batches {
		wg.Add(1)
		go func(b []EpisodePayload) {
			defer wg.Done()
			
			input := ExtractInput{Episodes: b}
			
			res, err := e.Housekeeper.Run(ctx, harness.StageExtract, input)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("extraction error: %w", err)
				}
				mu.Unlock()
				return
			}
			
			var output ExtractionResult
			if err := json.Unmarshal([]byte(res.Payload), &output); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to parse extraction output: %w", err)
				}
				mu.Unlock()
				return
			}
			
			output = e.enforceRules(output, b)
			
			mu.Lock()
			allResults = append(allResults, output)
			mu.Unlock()
		}(batch)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return allResults, nil
}
"""

code = re.sub(r'func \(e \*Extractor\) Extract.*?return allResults, nil\n\}', new_extract.strip(), code, flags=re.DOTALL)

with open("internal/core/pipeline/extract/extractor.go", "w") as f:
    f.write(code)
