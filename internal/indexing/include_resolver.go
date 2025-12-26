package indexing

import (
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

// resolveIncludesHeuristic parses #include "file" directives (quoted only) and records heuristic import refs.
func (gi *MasterIndex) resolveIncludesHeuristic(fileID types.FileID, path string, content []byte) {
	lines := strings.Split(string(content), "\n")
	baseDir := filepath.Dir(path)

	for i, line := range lines {
		lt := strings.TrimSpace(line)
		if !strings.HasPrefix(lt, "#include \"") {
			continue
		}

		rest := lt[len("#include \""):]
		end := strings.Index(rest, "\"")
		if end < 0 {
			continue
		}

		includeName := rest[:end]
		debug.LogIndexing("Heuristic include: %s -> %s (line %d)", path, includeName, i+1)

		var candidates []string
		rel := filepath.Clean(filepath.Join(baseDir, includeName))
		if gi.fileService.Exists(rel) {
			candidates = append(candidates, rel)
		}

		snap := gi.fileSnapshot.Load()
		for p := range snap.fileMap {
			if filepath.Base(p) == includeName && p != rel {
				candidates = append(candidates, p)
			}
		}

		resolved := len(candidates) == 1
		var resolvedPtr *bool
		resolvedVal := resolved
		unresolvedVal := false
		if resolved {
			resolvedPtr = &resolvedVal
		} else {
			resolvedPtr = &unresolvedVal
		}

		ref := types.Reference{
			FileID:         fileID,
			Line:           i + 1,
			Column:         1,
			Type:           types.RefTypeImport,
			ReferencedName: includeName,
			Quality:        "heuristic",
			Resolved:       resolvedPtr,
			Ambiguous:      !resolved && len(candidates) > 1,
			Candidates:     candidates,
		}

		if !resolved && len(candidates) == 0 {
			ref.FailureReason = "not_found"
		}

		gi.refTracker.AddHeuristicReference(ref)
	}
}
