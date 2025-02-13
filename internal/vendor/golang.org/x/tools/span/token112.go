// Copyright 2025 Upbound Inc.
// All rights reserved

//go:build go1.12
// +build go1.12

package span

import (
	"go/token"
)

// TODO(rstambler): Delete this file when we no longer support Go 1.11.
func lineStart(f *token.File, line int) token.Pos {
	return f.LineStart(line)
}
