// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package project

import (
	"github.com/upbound/up/cmd/up/project/build"
	"github.com/upbound/up/cmd/up/project/move"
	"github.com/upbound/up/cmd/up/project/push"
)

type Cmd struct {
	Init  initCmd   `cmd:"" help:"Initialize a new project."`
	Build build.Cmd `cmd:"" help:"Build a project into a Crossplane package."`
	Push  push.Cmd  `cmd:"" help:"Push a project's packages to the Upbound Marketplace."`
	Move  move.Cmd  `cmd:"" help:"Update the repository for a project"`
}
