// Copyright 2018 The Chubao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package cmd

import (
	"os"
	"github.com/chubaofs/chubaofs/cli/api"
	"github.com/spf13/cobra"
	"github.com/chubaofs/chubaofs/metanode"
	"fmt"
	"strconv"
	"github.com/chubaofs/chubaofs/util/log"
	"github.com/chubaofs/chubaofs/proto"
	"reflect"
)

const (
	cmdCompatibilityUse   = "compatibility"
	cmdCompatibilityShort = "compatibility test"
)

func newCompatibilityCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     cmdCompatibilityUse,
		Aliases: []string{"cptest"},
		Short:   cmdCompatibilityShort,
		Args:    cobra.MinimumNArgs(0),
	}
	cmd.AddCommand(
		newMetaCompatibilityCmd(),
	)
	return cmd
}

const (
	cmdMetaCompatibilityShort = "Verify metadata consistency  of meta partition"
)

func newMetaCompatibilityCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     CliOpMetaCompatibility,
		Short:   cmdMetaCompatibilityShort,
		Aliases: []string{"meta"},
		Args:    cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			var (
				snapshotPath = args[0]
				host         = args[1]
				pid          = args[2]
			)
			client := api.NewMetaHttpClient(host, false)
			defer func() {
				if err != nil {
					errout("Verify metadata consistency failed: %v\n", err)
					log.LogError(err)
					log.LogFlush()
					os.Exit(1)
				}
			}()
			id, err := strconv.ParseUint(pid, 10, 64)
			if err != nil {
				errout("parse pid[%v] failed: %v\n", pid, err)
				return
			}
			cursor, err := client.GetMetaPartition(id)
			if err != nil {
				return
			}
			mpcfg := &metanode.MetaPartitionConfig{
				Cursor:      cursor,
				PartitionId: id,
			}
			mp := metanode.NewMetaPartition(mpcfg, nil)
			err = mp.LoadSnapshot(snapshotPath)
			if err != nil {
				return
			}
			stdout("[Meta partition is %v, verify result]\n", id)
			if err = verifyDentry(client, mp); err != nil {
				stdout("%v\n", err)
				return
			}
			if err = verifyInode(client, mp); err != nil {
				stdout("%v\n", err)
				return
			}
			stdout("All meta has checked\n")
		},
	}
	return cmd
}

func verifyDentry(client *api.MetaHttpClient, mp metanode.MetaPartition) (err error) {
	dentryMap, err := client.GetAllDentry(mp.GetBaseConfig().PartitionId)
	if err != nil {
		return
	}
	mp.GetDentryTree().Ascend(func(d metanode.BtreeItem) bool {
		dentry, ok := d.(*metanode.Dentry)
		if !ok {
			stdout("item type is not *metanode.Dentry")
			err = fmt.Errorf("item type is not *metanode.Dentry")
			return false
		}
		key := fmt.Sprintf("%v_%v", dentry.ParentId, dentry.Name)
		oldDentry, ok := dentryMap[key]
		if !ok {
			stdout("dentry %v is not in old version", key)
			err = fmt.Errorf("dentry %v is not in old version", key)
			return false
		}
		if !reflect.DeepEqual(dentry, oldDentry) {
			stdout("dentry %v is not equal with old version", key)
			err = fmt.Errorf("dentry %v is not equal with old version,dentry[%v],oldDentry[%v]", key, dentry, oldDentry)
			return false
		}
		return true
	})
	stdout("The number of dentry is %v, all dentry are consistent \n", mp.GetDentryTree().Len())
	return
}

func verifyInode(client *api.MetaHttpClient, mp metanode.MetaPartition) (err error) {
	inodesMap, err := client.GetAllInodes(mp.GetBaseConfig().PartitionId)
	if err != nil {
		return
	}
	var localInode *api.Inode
	mp.GetInodeTree().Ascend(func(d metanode.BtreeItem) bool {
		inode, ok := d.(*metanode.Inode)
		if !ok {
			return true
		}
		oldInode, ok := inodesMap[inode.Inode]
		if !ok {
			stdout("inode %v is not in old version \n", inode.Inode)
			return true
		}
		localInode = &api.Inode{
			Inode:      inode.Inode,
			Type:       inode.Type,
			Uid:        inode.Uid,
			Gid:        inode.Gid,
			Size:       inode.Size,
			Generation: inode.Generation,
			CreateTime: inode.CreateTime,
			AccessTime: inode.AccessTime,
			ModifyTime: inode.ModifyTime,
			LinkTarget: inode.LinkTarget,
			NLink:      inode.NLink,
			Flag:       inode.Flag,
			Reserved:   inode.Reserved,
			Extents:    make([]proto.ExtentKey, 0),
		}
		inode.Extents.Range(func(ek proto.ExtentKey) bool {
			localInode.Extents = append(localInode.Extents, ek)
			return true
		})
		if !reflect.DeepEqual(oldInode, localInode) {
			stdout("inode %v is not equal with old version,inode[%v],oldInode[%v]\n", inode.Inode, inode, oldInode)
		}
		return true
	})
	stdout("The number of inodes is %v, all inodes are consistent \n", mp.GetInodeTree().Len())
	return
}
