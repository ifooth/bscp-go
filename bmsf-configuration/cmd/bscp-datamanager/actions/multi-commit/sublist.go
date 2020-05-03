/*
Tencent is pleased to support the open source community by making Blueking Container Service available.
Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
Licensed under the MIT License (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://opensource.org/licenses/MIT
Unless required by applicable law or agreed to in writing, software distributed under
the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied. See the License for the specific language governing permissions and
limitations under the License.
*/

package multicommit

import (
	"errors"

	"github.com/spf13/viper"

	"bk-bscp/internal/database"
	"bk-bscp/internal/dbsharding"
	pbcommon "bk-bscp/internal/protocol/common"
	pb "bk-bscp/internal/protocol/datamanager"
)

// SubListAction is multi commit sub list action.
type SubListAction struct {
	viper *viper.Viper
	smgr  *dbsharding.ShardingManager

	req  *pb.QueryMultiCommitSubListReq
	resp *pb.QueryMultiCommitSubListResp

	sd *dbsharding.ShardingDB

	commits []database.Commit
}

// NewSubListAction creates new SubListAction.
func NewSubListAction(viper *viper.Viper, smgr *dbsharding.ShardingManager,
	req *pb.QueryMultiCommitSubListReq, resp *pb.QueryMultiCommitSubListResp) *SubListAction {
	action := &SubListAction{viper: viper, smgr: smgr, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.ErrCode = pbcommon.ErrCode_E_OK
	action.resp.ErrMsg = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *SubListAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.ErrCode = errCode
	act.resp.ErrMsg = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *SubListAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Output handles the output messages.
func (act *SubListAction) Output() error {
	commitids := []string{}
	for _, st := range act.commits {
		commitids = append(commitids, st.Commitid)
	}
	act.resp.Commitids = commitids
	return nil
}

func (act *SubListAction) verify() error {
	length := len(act.req.Bid)
	if length == 0 {
		return errors.New("invalid params, bid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, bid too long")
	}

	length = len(act.req.MultiCommitid)
	if length == 0 {
		return errors.New("invalid params, multi commitid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, multi commitid too long")
	}
	return nil
}

func (act *SubListAction) querySubCommits() (pbcommon.ErrCode, string) {
	act.sd.AutoMigrate(&database.Commit{})

	// selected fields.
	fields := "Fid, Fcommitid, Fbid, Fappid, Fcfgsetid, Ftemplateid, Fop, " +
		"Foperator, Freleaseid, Fmemo, Fstate, Fcreate_time, Fupdate_time"

	err := act.sd.DB().
		Select(fields).
		Order("Fupdate_time DESC").
		Where(&database.Commit{Bid: act.req.Bid, MultiCommitid: act.req.MultiCommitid}).
		Find(&act.commits).Error

	if err != nil {
		return pbcommon.ErrCode_E_DM_DB_EXEC_ERR, err.Error()
	}
	return pbcommon.ErrCode_E_OK, ""
}

// Do makes the workflows of this action base on input messages.
func (act *SubListAction) Do() error {
	// business sharding db.
	sd, err := act.smgr.ShardingDB(act.req.Bid)
	if err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_ERR_DBSHARDING, err.Error())
	}
	act.sd = sd

	// query sub commits.
	if errCode, errMsg := act.querySubCommits(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
