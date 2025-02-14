/*
Copyright (c) 2016 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package account

import (
	"context"
	"flag"

	"github.com/vmware/govmomi/cli"
)

type remove struct {
	*AccountFlag
}

func init() {
	cli.Register("host.account.remove", &remove{})
}

func (cmd *remove) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.AccountFlag, ctx = newAccountFlag(ctx)
	cmd.AccountFlag.Register(ctx, f)
}

func (cmd *remove) Description() string {
	return `Remove local account on HOST.

Examples:
  govc host.account.remove -id $USER`
}

func (cmd *remove) Process(ctx context.Context) error {
	if err := cmd.AccountFlag.Process(ctx); err != nil {
		return err
	}
	return nil
}

func (cmd *remove) Run(ctx context.Context, f *flag.FlagSet) error {
	m, err := cmd.AccountFlag.HostAccountManager(ctx)
	if err != nil {
		return err
	}
	return m.Remove(ctx, cmd.HostAccountSpec.Id)
}
