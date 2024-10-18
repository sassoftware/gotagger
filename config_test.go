package gotagger

import (
	"testing"

	"github.com/sassoftware/gotagger/mapper"
	"github.com/stretchr/testify/assert"
)

func TestConfig_ParseJSON(t *testing.T) {
	tests := []struct {
		title           string
		commitTypeTable mapper.Table
		configFileData  string
		wantErr         string
		want            Config
	}{
		{
			title:           "no config",
			commitTypeTable: mapper.Table{},
			configFileData:  "",
			wantErr:         "unexpected end of JSON input",
		},
		{
			title: "good config",
			configFileData: `{
	"incrementMappings": {
		"feat": "minor",
		"fix": "patch",
		"refactor": "patch",
		"perf": "patch",
		"test": "patch",
		"style": "patch",
		"build": "none",
		"chore": "none",
		"ci": "none",
		"docs": "none",
		"revert": "none"
	},
	"defaultIncrement": "none"
}`,
			want: Config{
				RemoteName:    "origin",
				VersionPrefix: "v",
				CommitTypeTable: mapper.NewTable(
					mapper.Mapper{
						mapper.TypeFeature:     mapper.IncrementMinor,
						mapper.TypeBugFix:      mapper.IncrementPatch,
						mapper.TypeRefactor:    mapper.IncrementPatch,
						mapper.TypePerformance: mapper.IncrementPatch,
						mapper.TypeTest:        mapper.IncrementPatch,
						mapper.TypeStyle:       mapper.IncrementPatch,
						mapper.TypeBuild:       mapper.IncrementNone,
						mapper.TypeChore:       mapper.IncrementNone,
						mapper.TypeCI:          mapper.IncrementNone,
						mapper.TypeDocs:        mapper.IncrementNone,
						mapper.TypeRevert:      mapper.IncrementNone,
					},
					mapper.IncrementNone,
				),
			},
		},
		{
			title: "duplicate mapping",
			configFileData: `{
	"incrementMappings": {
		"feat": "minor",
		"feat": "patch"
	},
	"defaultIncrement": "none"
}`,
			want: Config{
				RemoteName:    "origin",
				VersionPrefix: "v",
				CommitTypeTable: mapper.NewTable(
					mapper.Mapper{
						mapper.TypeFeature: mapper.IncrementPatch,
					},
					mapper.IncrementNone,
				),
			},
		},
		{
			title: "unknown commit type",
			configFileData: `{
	"incrementMappings": {
		"feet": "minor"
	},
	"defaultIncrement": "none"
}`,
			want: Config{
				RemoteName:    "origin",
				VersionPrefix: "v",
				CommitTypeTable: mapper.NewTable(
					mapper.Mapper{
						"feet": mapper.IncrementMinor,
					},
					mapper.IncrementNone,
				),
			},
		},
		{
			title: "release not allowed",
			configFileData: `{
	"incrementMappings": {
		"release": "minor"
	}
}`,
			wantErr: "release mapping is not allowed",
		},
		{
			title: "attempt major increment",
			configFileData: `{
	"incrementMappings": {
		"feat": "major"
	}
}`,
			wantErr: "major version increments cannot be mapped to commit types. use the commit spec directives for this",
		},
		{
			title: "no default",
			configFileData: `{
	"incrementMappings": {
		"feat": "minor"
	}
}`,
			want: Config{
				RemoteName:    "origin",
				VersionPrefix: "v",
				CommitTypeTable: mapper.NewTable(
					mapper.Mapper{
						mapper.TypeFeature: mapper.IncrementMinor,
					},
					mapper.IncrementPatch,
				),
			},
		},
		{
			title: "invalid increment",
			configFileData: `{
	"incrementMappings": {
		"feat": "supermajor"
	},
	"defaultIncrement": "none"
}`,
			wantErr: "invalid version increment 'supermajor'",
		},
		{
			title:          "invalid json",
			configFileData: "{ this is bad json",
			wantErr:        "invalid character 't' looking for beginning of object key string",
		},
		{
			title:          "empty version prefix",
			configFileData: `{"versionPrefix":""}`,
			want: Config{
				RemoteName: "origin",
				CommitTypeTable: mapper.NewTable(
					mapper.Mapper{
						mapper.TypeFeature: mapper.IncrementMinor,
					},
					mapper.IncrementPatch,
				),
			},
		},
		{
			title:          "major dirty worktree increment",
			configFileData: `{"incrementDirtyWorktree": "major"}`,
			wantErr:        "major version increments are not allowed for dirty worktrees",
		},
		{
			title:          "default config",
			configFileData: `{}`,
			want: Config{
				CreateTag:              false,
				ExcludeModules:         nil,
				IgnoreModules:          false,
				RemoteName:             "origin",
				PreMajor:               false,
				PushTag:                false,
				VersionPrefix:          "v",
				DirtyWorktreeIncrement: mapper.IncrementNone,
				CommitTypeTable:        mapper.NewTable(mapper.Mapper{"feat": mapper.IncrementMinor}, mapper.IncrementPatch),
				Force:                  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()
			cfg := NewDefaultConfig()

			err := cfg.ParseJSON([]byte(tt.configFileData))
			if tt.wantErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, cfg)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
