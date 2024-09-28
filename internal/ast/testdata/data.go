package testdata

import _ "embed"

// FmtInput is a canonical fmt'd file, the 'Bad' ones should resolve to this
// output, and it should resolve to itself.

//go:embed fmt_canon.bcl
var FmtInput string

//go:embed fmt_bad_1.bcl
var FmtBad1 string
