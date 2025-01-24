// Copyright 2025 Upbound Inc.
// All rights reserved

package feature

import (
	"github.com/alecthomas/kong"
)

// maturityTag is the struct field tag used to specify maturity of a command.
const maturityTag = "maturity"

// Maturity is the maturity of a feature.
type Maturity string

// Currently supported maturity levels.
const (
	Alpha      Maturity = "alpha"
	Stable     Maturity = "stable"
	Deprecated Maturity = "deprecated"
)

// HideMaturity hides commands that are not at the specified level of maturity.
func HideMaturity(p *kong.Path, maturity Maturity) error {
	nodes := p.Node().Children // copy to avoid possibility of reslicing
	nodes = append(nodes, p.Node())
	for _, c := range nodes {
		mt := Maturity(c.Tag.Get(maturityTag))
		if mt == "" {
			mt = Stable
		}
		if mt != maturity {
			c.Hidden = true
		}
	}
	return nil
}

// GetMaturity gets the maturity of the node.
func GetMaturity(n *kong.Node) Maturity {
	if m := Maturity(n.Tag.Get(maturityTag)); m != "" {
		return m
	}
	return Stable
}
