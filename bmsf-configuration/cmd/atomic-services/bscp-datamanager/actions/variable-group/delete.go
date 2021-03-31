/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.,
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under,
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package variablegroup

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/viper"

	"bk-bscp/internal/database"
	"bk-bscp/internal/dbsharding"
	pbcommon "bk-bscp/internal/protocol/common"
	pb "bk-bscp/internal/protocol/datamanager"
	"bk-bscp/pkg/common"
)

// DeleteAction action for delete config template.
type DeleteAction struct {
	ctx   context.Context
	viper *viper.Viper
	smgr  *dbsharding.ShardingManager

	req  *pb.DeleteVariableGroupReq
	resp *pb.DeleteVariableGroupResp

	sd *dbsharding.ShardingDB
}

// NewDeleteAction create new DeleteAction.
func NewDeleteAction(ctx context.Context, viper *viper.Viper, smgr *dbsharding.ShardingManager,
	req *pb.DeleteVariableGroupReq, resp *pb.DeleteVariableGroupResp) *DeleteAction {
	action := &DeleteAction{ctx: ctx, viper: viper, smgr: smgr, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.Code = pbcommon.ErrCode_E_OK
	action.resp.Message = "OK"

	return action
}

// Err setup error code message in response and return error
func (act *DeleteAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.Code = errCode
	act.resp.Message = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages
func (act *DeleteAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Output handles the output messages.
func (act *DeleteAction) Output() error {
	// do nothing.
	return nil
}

func (act *DeleteAction) verify() error {
	var err error

	if err = common.ValidateString("biz_id", act.req.BizId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("var_group_id", act.req.VarGroupId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("operator", act.req.Operator,
		database.BSCPNOTEMPTY, database.BSCPNAMELENLIMIT); err != nil {
		return err
	}
	return nil
}

func (act *DeleteAction) queryVariablesCount() (int64, pbcommon.ErrCode, string) {
	var totalCount int64

	err := act.sd.DB().
		Model(&database.Variable{}).
		Where(&database.Variable{BizID: act.req.BizId, VarGroupID: act.req.VarGroupId}).
		Count(&totalCount).Error

	if err != nil {
		return 0, pbcommon.ErrCode_E_DM_DB_EXEC_ERR, err.Error()
	}
	return totalCount, pbcommon.ErrCode_E_OK, "OK"
}

func (act *DeleteAction) deleteVariableGroup() (pbcommon.ErrCode, string) {
	if len(act.req.VarGroupId) == 0 {
		return pbcommon.ErrCode_E_DM_PARAMS_INVALID, "can't delete resource without var_group_id"
	}

	// check variables count of the group.
	variablesCount, errCode, errMsg := act.queryVariablesCount()
	if errCode != pbcommon.ErrCode_E_OK {
		return errCode, fmt.Sprintf("check variables count failed, %s", errMsg)
	}
	if variablesCount != 0 {
		return pbcommon.ErrCode_E_DM_PARAMS_INVALID, "can't delete variable group which has left variables"
	}

	// delete.
	exec := act.sd.DB().
		Limit(1).
		Where(&database.VariableGroup{BizID: act.req.BizId, VarGroupID: act.req.VarGroupId}).
		Delete(&database.VariableGroup{})

	if err := exec.Error; err != nil {
		return pbcommon.ErrCode_E_DM_DB_EXEC_ERR, err.Error()
	}
	if exec.RowsAffected == 0 {
		return pbcommon.ErrCode_E_DM_DB_ROW_AFFECTED_ERR,
			"delete variable group failed, there is no variable group fit in conditions"
	}
	return pbcommon.ErrCode_E_OK, "OK"
}

// Do makes the workflows of this action base on input messages.
func (act *DeleteAction) Do() error {
	// business sharding db.
	sd, err := act.smgr.ShardingDB(act.req.BizId)
	if err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_ERR_DBSHARDING, err.Error())
	}
	act.sd = sd

	// delete variable group.
	if errCode, errMsg := act.deleteVariableGroup(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
