package parser

import "github.com/TrNgTien/vfs/internal/parser/sig"

type Stats struct {
	FilesScanned  int
	FilesMatched  int
	RawBytes      int64
	RawLines      int
	VFSBytes      int
	VFSLines      int
	ExportedFuncs int
}

type FileResult struct {
	RelPath  string
	Sigs     []sig.Sig
	RawBytes int64
	RawLines int
}

func ComputeStats(results []FileResult) Stats {
	var st Stats
	st.FilesMatched = len(results)
	for _, r := range results {
		st.RawBytes += r.RawBytes
		st.RawLines += r.RawLines
		for _, s := range r.Sigs {
			line := s.FormatLine(r.RelPath)
			st.VFSBytes += len(line) + 1
			st.VFSLines++
		}
		st.ExportedFuncs += len(r.Sigs)
	}
	return st
}
