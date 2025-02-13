// Copyright 2025 Upbound Inc.
// All rights reserved

package undo

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func ExampleDo() {
	fmt.Println(Do(func(u Undoer) error {
		u.Undo(func() error {
			fmt.Println("undoing")
			return errors.New("undo error")
		})
		fmt.Println("doing")
		return errors.New("do error")
	}))
	// Output:
	// doing
	// undoing
	// [do error, undo error]
}
