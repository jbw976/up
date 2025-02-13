// Copyright 2025 Upbound Inc.
// All rights reserved

package query

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAllowedFormats(t *testing.T) {
	formatsInTag := extractFormatsFromHelpTag()
	formatsInPrinter := printFlags.AllowedFormats()

	if diff := cmp.Diff(formatsInPrinter, formatsInTag); diff != "" {
		t.Errorf("QueryCmd{}.OutputFormats: -want err, +got err:\n%s\nexpected: %s", diff, strings.Join(formatsInPrinter, ","))
	}
}

func extractFormatsFromHelpTag() []string {
	t := reflect.TypeOf(QueryCmd{})
	field, found := t.FieldByName("OutputFormat")
	if !found {
		return nil
	}

	helpTag := field.Tag.Get("help")
	prefix := "One of: "
	i := strings.Index(helpTag, prefix)
	if i == -1 {
		return nil
	}

	formats := helpTag[i+len(prefix):]
	return strings.Split(formats, ",")
}
