// Copyright 2025 Upbound Inc.
// All rights reserved

package config

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/profile"
)

func TestAddOrUpdateUpboundProfile(t *testing.T) {
	name := "cool-profile"
	profOne := profile.Profile{
		ID:        "cool-user",
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
	}
	profTwo := profile.Profile{
		ID:        "cool-user",
		TokenType: profile.TokenTypeUser,
		Account:   "other-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		add    profile.Profile
		want   *Config
		err    error
	}{
		"AddNewProfile": {
			reason: "Adding a new profile to an empty Config should not cause an error.",
			name:   name,
			cfg:    &Config{},
			add:    profOne,
			want: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{name: profOne},
				},
			},
		},
		"UpdateExistingProfile": {
			reason: "Updating an existing profile in the Config should not cause an error.",
			name:   name,
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{name: profOne},
				},
			},
			add: profTwo,
			want: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{name: profTwo},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.AddOrUpdateUpboundProfile(tc.name, tc.add)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAddOrUpdateUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, tc.cfg); diff != "" {
				t.Errorf("\n%s\nAddOrUpdateUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDeleteUpboundProfile(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		reason string
		cfg    *Config
		name   string
		err    error
		want   *Config
	}{
		"NilProfiles": {
			reason: "If the profiles map is nil, an error should be returned.",
			cfg:    &Config{},
			name:   "default",
			err:    errors.Errorf(errProfileNotFoundFmt, "default"),
			want:   &Config{},
		},
		"EmptyProfiles": {
			reason: "If there are no profiles, an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{},
				},
			},
			name: "default",
			err:  errors.Errorf(errProfileNotFoundFmt, "default"),
			want: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{},
				},
			},
		},
		"NotFound": {
			reason: "If the profile is not found, an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"not-default": {},
					},
				},
			},
			name: "default",
			err:  errors.Errorf(errProfileNotFoundFmt, "default"),
			want: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"not-default": {},
					},
				},
			},
		},
		"DefaultProfile": {
			reason: "If the profile is the default profile, it should be deleted and the default updated.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
			name: "default",
			want: &Config{
				Upbound: Upbound{
					Default: "not-default",
					Profiles: map[string]profile.Profile{
						"not-default": {},
					},
				},
			},
		},
		"NonDefaultProfile": {
			reason: "If the profile is not the default, it should be deleted and the default unchanged.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
			name: "not-default",
			want: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default": {},
					},
				},
			},
		},
		"LastProfile": {
			reason: "If the profile is the last profile in the config, it should be deleted and the default unset.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default": {},
					},
				},
			},
			name: "default",
			want: &Config{
				Upbound: Upbound{
					Default:  "",
					Profiles: map[string]profile.Profile{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.DeleteUpboundProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDeleteUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, tc.cfg); diff != "" {
				t.Errorf("\n%s\nDeleteUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRenameUpboundProfile(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		reason string
		cfg    *Config
		from   string
		to     string
		err    error
		want   *Config
	}{
		"NilProfiles": {
			reason: "If the profiles map is nil, an error should be returned.",
			cfg:    &Config{},
			from:   "default",
			to:     "not-default",
			err:    errors.Errorf(errProfileNotFoundFmt, "default"),
			want:   &Config{},
		},
		"EmptyProfiles": {
			reason: "If there are no profiles, an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{},
				},
			},
			from: "default",
			to:   "not-default",
			err:  errors.Errorf(errProfileNotFoundFmt, "default"),
			want: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{},
				},
			},
		},
		"NotFound": {
			reason: "If the profile is not found, an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "not-default",
					Profiles: map[string]profile.Profile{
						"not-default": {},
					},
				},
			},
			from: "default",
			to:   "not-default",
			err:  errors.Errorf(errProfileNotFoundFmt, "default"),
			want: &Config{
				Upbound: Upbound{
					Default: "not-default",
					Profiles: map[string]profile.Profile{
						"not-default": {},
					},
				},
			},
		},
		"Overwrite": {
			reason: "If the new profile name already exists, an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
			from: "default",
			to:   "not-default",
			err:  errors.Errorf(errProfileAlreadyExistsFmt, "not-default"),
			want: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
		},
		"NoOp": {
			reason: "If from and to are the same, the config is unchanged.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
			from: "default",
			to:   "default",
			want: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
		},
		"NonDefaultProfile": {
			reason: "If the profile is not the default, it should be renamed and the default unchanged.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
			from: "not-default",
			to:   "new-profile",
			want: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"new-profile": {},
					},
				},
			},
		},
		"DefaultProfile": {
			reason: "If the profile is the default, it should be renamed and the default updated.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default":     {},
						"not-default": {},
					},
				},
			},
			from: "default",
			to:   "new-profile",
			want: &Config{
				Upbound: Upbound{
					Default: "new-profile",
					Profiles: map[string]profile.Profile{
						"new-profile": {},
						"not-default": {},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.RenameUpboundProfile(tc.from, tc.to)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDeleteUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, tc.cfg); diff != "" {
				t.Errorf("\n%s\nDeleteUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetDefaultUpboundProfile(t *testing.T) {
	name := "cool-profile"
	profOne := profile.Profile{
		ID:        "cool-user",
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		want   profile.Profile
		err    error
	}{
		"ErrorNoDefault": {
			reason: "If no default defined an error should be returned.",
			cfg:    &Config{},
			want:   profile.Profile{},
			err:    errors.New(errNoDefaultSpecified),
		},
		"ErrorDefaultNotExist": {
			reason: "If defined default does not exist an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "test",
				},
			},
			want: profile.Profile{},
			err:  errors.New(errDefaultNotExist),
		},
		"Successful": {
			reason: "If defined default exists it should be returned.",
			name:   name,
			cfg: &Config{
				Upbound: Upbound{
					Default:  "cool-profile",
					Profiles: map[string]profile.Profile{name: profOne},
				},
			},
			want: profOne,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			name, prof, err := tc.cfg.GetDefaultUpboundProfile()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetDefaultUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.name, name); diff != "" {
				t.Errorf("\n%s\nGetDefaultUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, prof); diff != "" {
				t.Errorf("\n%s\nGetDefaultUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetUpboundProfile(t *testing.T) {
	name := "cool-profile"
	profOne := profile.Profile{
		ID:        "cool-user",
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		want   profile.Profile
		err    error
	}{
		"ErrorProfileNotExist": {
			reason: "If profile does not exist an error should be returned.",
			name:   name,
			cfg:    &Config{},
			want:   profile.Profile{},
			err:    errors.Errorf(errProfileNotFoundFmt, "cool-profile"),
		},
		"Successful": {
			reason: "If profile exists it should be returned.",
			name:   "cool-profile",
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{name: profOne},
				},
			},
			want: profOne,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			prof, err := tc.cfg.GetUpboundProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, prof); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetDefaultUpboundProfile(t *testing.T) {
	name := "cool-user"
	profOne := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		err    error
	}{
		"ErrorProfileNotExist": {
			reason: "If profile does not exist an error should be returned.",
			name:   name,
			cfg:    &Config{},
			err:    errors.Errorf(errProfileNotFoundFmt, "cool-user"),
		},
		"Successful": {
			reason: "If profile exists it should be set as default.",
			name:   "cool-user",
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]profile.Profile{name: profOne},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.SetDefaultUpboundProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetUpboundProfiles(t *testing.T) {
	nameOne := "cool-user"
	profOne := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
	}
	nameTwo := "cool-user2"
	profTwo := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org2",
	}

	type args struct {
		cfg *Config
	}
	type want struct {
		err      error
		profiles map[string]profile.Profile
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				cfg: &Config{},
			},
			want: want{
				err: errors.New(errNoProfilesFound),
			},
		},
		"Successful": {
			reason: "If profile exists it should be set as default.",
			args: args{
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]profile.Profile{
							nameOne: profOne,
							nameTwo: profTwo,
						},
					},
				},
			},
			want: want{
				profiles: map[string]profile.Profile{
					nameOne: profOne,
					nameTwo: profTwo,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			profiles, err := tc.args.cfg.GetUpboundProfiles()

			if diff := cmp.Diff(tc.want.profiles, profiles); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfiles(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfiles(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetBaseConfig(t *testing.T) {
	nameOne := "cool-user"
	profOne := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
		BaseConfig: map[string]string{
			"key": "value",
		},
	}
	nameTwo := "cool-user2"
	profTwo := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org2",
	}

	type args struct {
		profile string
		cfg     *Config
	}
	type want struct {
		err  error
		base map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				profile: nameTwo,
				cfg:     &Config{},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, nameTwo),
			},
		},
		"Successful": {
			reason: "If profile exists, its base config should be returned.",
			args: args{
				profile: nameOne,
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]profile.Profile{
							nameOne: profOne,
							nameTwo: profTwo,
						},
					},
				},
			},
			want: want{
				base: profOne.BaseConfig,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			base, err := tc.args.cfg.GetBaseConfig(tc.args.profile)

			if diff := cmp.Diff(tc.want.base, base); diff != "" {
				t.Errorf("\n%s\nGetBaseConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetBaseConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAddToBaseConfig(t *testing.T) {
	nameOne := "cool-user"
	profOne := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org",
	}
	nameTwo := "cool-user2"
	profTwo := profile.Profile{
		TokenType: profile.TokenTypeUser,
		Account:   "cool-org2",
	}

	type args struct {
		profile string
		key     string
		value   string
		cfg     *Config
	}
	type want struct {
		err  error
		base map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				profile: nameTwo,
				cfg:     &Config{},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, nameTwo),
			},
		},
		"Successful": {
			reason: "If profile exists, we should add the k,v pair to the base config.",
			args: args{
				profile: nameOne,
				key:     "k",
				value:   "v",
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]profile.Profile{
							nameOne: profOne,
							nameTwo: profTwo,
						},
					},
				},
			},
			want: want{
				base: map[string]string{
					"k": "v",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cfg.AddToBaseConfig(tc.args.profile, tc.args.key, tc.args.value)
			base, _ := tc.args.cfg.GetBaseConfig(tc.args.profile)

			if diff := cmp.Diff(tc.want.base, base); diff != "" {
				t.Errorf("\n%s\nAddToBaseConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAddToBaseConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBaseToJSON(t *testing.T) {
	dneName := "does not exist"
	exists := "exists"

	type args struct {
		profile string
		cfg     *Config
	}
	type want struct {
		err  error
		base string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				profile: dneName,
				cfg:     &Config{},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, dneName),
			},
		},
		"Successful": {
			reason: "If profile exists, we should add the k,v pair to the base config.",
			args: args{
				profile: exists,
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]profile.Profile{
							exists: {
								TokenType: profile.TokenTypeUser,
								Account:   "account",
								BaseConfig: map[string]string{
									"k": "v",
								},
							},
						},
					},
				},
			},
			want: want{
				base: "{\"k\":\"v\"}\n",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := tc.args.cfg.BaseToJSON(tc.args.profile)
			if r != nil {
				base, _ := io.ReadAll(r)
				if diff := cmp.Diff(tc.want.base, string(base)); diff != "" {
					t.Errorf("\n%s\nBaseToJSON(...): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBaseToJSON(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
