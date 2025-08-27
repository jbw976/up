// Copyright 2025 Upbound Inc.
// All rights reserved

package report

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
)

func TestNextReportDirName(t *testing.T) {
	type args struct {
		existingFiles []string
	}
	type want struct {
		err    error
		result string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyFilesystem": {
			reason: "Should return report0 when filesystem is empty.",
			args: args{
				existingFiles: []string{},
			},
			want: want{
				result: "report0",
			},
		},
		"NoReportDirectories": {
			reason: "Should return report0 when no report directories exist.",
			args: args{
				existingFiles: []string{
					"junk",
				},
			},
			want: want{
				result: "report0",
			},
		},
		"OneReportDirectory": {
			reason: "Should return report1 when a report directory already exists.",
			args: args{
				existingFiles: []string{
					"report0/usage.json",
					"report0/meta.json",
				},
			},
			want: want{
				result: "report1",
			},
		},
		"MultipleReportDirectories": {
			reason: "Should return report3 when three report directories exist.",
			args: args{
				existingFiles: []string{
					"report0/usage.json",
					"report1/usage.json",
					"report2/usage.json",
				},
			},
			want: want{
				result: "report3",
			},
		},
		"MixedReportNames": {
			reason: "Should count all files starting with 'report' prefix.",
			args: args{
				existingFiles: []string{
					"reports",
					"report/data1.csv",
					"report0/data2.csv",
					"report1/data3.csv",
					"report-backup/old.csv",
					"report_temp/temp.csv",
					"reportabc/other.csv",
				},
			},
			want: want{
				result: "report7",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Populate the filesystem with existing files and directories.
			fs := afero.NewMemMapFs()
			for _, filePath := range tc.args.existingFiles {
				f, err := fs.Create("/" + filePath)
				if err != nil {
					t.Fatalf("Failed to create file %s: %v", filePath, err)
				}
				f.Close()
			}

			result, err := nextReportDirName(fs)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nnextReportDirName(...): -want err, +got err\n%s", name, diff)
			}

			if diff := cmp.Diff(tc.want.result, result); diff != "" {
				t.Errorf("%s\nnextReportDirName(...): -want, +got\n%s", name, diff)
			}
		})
	}
}
