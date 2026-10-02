awk '
/func isValidEvidence/ { flag = 1 }
flag && /^}/ { print "}\n"; flag = 0; next }
flag { next }
!flag { print }
' internal/core/pipeline/extract/extractor.go > temp.go

cat << 'EOF2' >> temp.go
func isValidEvidence(ev claims.Evidence, batch []EpisodePayload) bool {
	if ev.Kind != "human" && ev.Kind != "tool_outcome" {
		return false
	}
	return true
}
EOF2
mv temp.go internal/core/pipeline/extract/extractor.go
