// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/vmware/govmomi/cli"
	"github.com/vmware/govmomi/cli/flags"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/units"
	"github.com/vmware/govmomi/vim25/types"
)

type ls struct {
	*flags.DatastoreFlag
	all      bool
	long     bool
	path     bool
	r        bool
	category string
	tag      string
	tags     bool
}

func init() {
	cli.Register("disk.ls", &ls{})
}

func (cmd *ls) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.DatastoreFlag, ctx = flags.NewDatastoreFlag(ctx)
	cmd.DatastoreFlag.Register(ctx, f)

	f.BoolVar(&cmd.all, "a", false, "List IDs with missing file backing")
	f.BoolVar(&cmd.long, "l", false, "Long listing format")
	f.BoolVar(&cmd.path, "L", false, "Print disk backing path instead of disk name")
	f.BoolVar(&cmd.r, "R", false, "Reconcile the datastore inventory info")
	f.StringVar(&cmd.category, "c", "", "Query tag category")
	f.StringVar(&cmd.tag, "t", "", "Query tag name")
	f.BoolVar(&cmd.tags, "T", false, "List attached tags")
}

func (cmd *ls) Usage() string {
	return "[ID]..."
}

func (cmd *ls) Description() string {
	return `List disk IDs on DS.

Examples:
  govc disk.ls
  govc disk.ls -l -T
  govc disk.ls -l e9b06a8b-d047-4d3c-b15b-43ea9608b1a6
  govc disk.ls -c k8s-region -t us-west-2`
}

type VStorageObject struct {
	types.VStorageObject
	Tags []types.VslmTagEntry `json:"tags"`
}

func (o *VStorageObject) tags() string {
	var tags []string
	for _, tag := range o.Tags {
		tags = append(tags, tag.ParentCategoryName+":"+tag.TagName)
	}
	return strings.Join(tags, ",")
}

type lsResult struct {
	cmd     *ls
	Objects []VStorageObject `json:"objects"`
}

func (r *lsResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(r.cmd.Out, 2, 0, 2, ' ', 0)

	for _, o := range r.Objects {
		name := o.Config.Name
		if r.cmd.path {
			if file, ok := o.Config.Backing.(*types.BaseConfigInfoDiskFileBackingInfo); ok {
				name = file.FilePath
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s", o.Config.Id.Id, name)
		if r.cmd.long {
			created := o.Config.CreateTime.Format(time.Stamp)
			size := units.FileSize(o.Config.CapacityInMB * 1024 * 1024)
			_, _ = fmt.Fprintf(tw, "\t%s\t%s", size, created)
		}
		if r.cmd.tags {
			_, _ = fmt.Fprintf(tw, "\t%s", o.tags())
		}
		_, _ = fmt.Fprintln(tw)
	}

	return tw.Flush()
}

func (r *lsResult) Dump() interface{} {
	return r.Objects
}

func (cmd *ls) Run(ctx context.Context, f *flag.FlagSet) error {
	m, err := NewManagerFromFlag(ctx, cmd.DatastoreFlag)
	if err != nil {
		return err
	}

	if cmd.r {
		if err = m.ReconcileDatastoreInventory(ctx); err != nil {
			return err
		}
	}
	res := lsResult{cmd: cmd}

	filterNotFound := false
	ids := f.Args()
	if len(ids) == 0 {
		filterNotFound = true
		var oids []types.ID
		if cmd.category == "" {
			oids, err = m.List(ctx)
		} else {
			oids, err = m.ListAttachedObjects(ctx, cmd.category, cmd.tag)
		}

		if err != nil {
			return err
		}
		for _, id := range oids {
			ids = append(ids, id.Id)
		}
	}

	for _, id := range ids {
		o, err := m.Retrieve(ctx, id)
		if err != nil {
			if filterNotFound && fault.Is(err, &types.NotFound{}) {
				// The case when an FCD is deleted by something other than DeleteVStorageObject_Task, such as VM destroy
				if cmd.all {
					obj := VStorageObject{VStorageObject: types.VStorageObject{
						Config: types.VStorageObjectConfigInfo{
							BaseConfigInfo: types.BaseConfigInfo{
								Id:   types.ID{Id: id},
								Name: "not found: use 'disk.ls -R' to reconcile datastore inventory",
							},
						},
					}}
					res.Objects = append(res.Objects, obj)
				}
				continue
			}
			return fmt.Errorf("retrieve %q: %s", id, err)
		}

		obj := VStorageObject{VStorageObject: *o}
		if cmd.tags {
			obj.Tags, err = m.ListAttachedTags(ctx, id)
			if err != nil {
				return err
			}
		}
		res.Objects = append(res.Objects, obj)
	}

	return cmd.WriteResult(&res)
}
