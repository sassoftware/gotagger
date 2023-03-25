// Copyright © 2020, SAS Institute Inc., Cary, NC, USA.  All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		title         string
		incrementType string
		want          Increment
		wantErr       string
	}{
		{
			title:         "major",
			incrementType: "major",
			want:          IncrementMajor,
			wantErr:       "",
		},
		{
			title:         "minor",
			incrementType: "minor",
			want:          IncrementMinor,
			wantErr:       "",
		},
		{
			title:         "patch",
			incrementType: "patch",
			want:          IncrementPatch,
			wantErr:       "",
		},
		{
			title:         "none",
			incrementType: "none",
			want:          IncrementNone,
			wantErr:       "",
		},
		{
			title:         "none empty",
			incrementType: "",
			want:          IncrementNone,
			wantErr:       "",
		},
		{
			title:         "invalid type",
			incrementType: "fake",
			want:          IncrementNone,
			wantErr:       "invalid version increment 'fake'",
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()
			got, err := Convert(tt.incrementType)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestTypeTable_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		index string
		table Table
		want  Increment
	}{
		{
			name:  "table contains",
			index: TypeBugFix,
			table: Table{
				Mapper:     defaultCommitTypeMapper,
				defaultInc: IncrementPatch,
			},
			want: IncrementPatch,
		},
		{
			name:  "table does not contain",
			index: "fake",
			table: Table{
				Mapper:     defaultCommitTypeMapper,
				defaultInc: IncrementPatch,
			},
			want: IncrementPatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t1 *testing.T) {
			inc := tt.table.Get(tt.index)
			assert.Equal(t, tt.want, inc)
		})
	}
}
