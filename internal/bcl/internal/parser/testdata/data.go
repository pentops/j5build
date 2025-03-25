package testdata

import _ "embed"

// FmtInput is a canonical fmt'd file, the 'Bad' ones should resolve to this
// output, and it should resolve to itself.

// Using txt file ext to stop the LSP from registering this as a bcl file

//go:embed fmt_canon.txt
var FmtInput string

//go:embed fmt_bad_1.txt
var FmtBad1 string
