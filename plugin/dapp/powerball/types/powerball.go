// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"reflect"

	"github.com/33cn/chain33/common/address"
	log "github.com/33cn/chain33/common/log/log15"
	"github.com/33cn/chain33/types"
)

var (
	plog = log.New("module", "exectype."+PowerballX)
)

func init() {
	types.AllowUserExec = append(types.AllowUserExec, []byte(PowerballX))
	types.RegistorExecutor(PowerballX, NewType())
	types.RegisterDappFork(PowerballX, "Enable", 0)
}

type PowerballType struct {
	types.ExecTypeBase
}

func NewType() *PowerballType {
	c := &PowerballType{}
	c.SetChild(c)
	return c
}

func (at *PowerballType) GetLogMap() map[int64]*types.LogInfo {
	return map[int64]*types.LogInfo{
		TyLogPowerballCreate: {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballCreate"},
		TyLogPowerballBuy:    {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballBuy"},
		TyLogPowerballDraw:   {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballDraw"},
		TyLogPowerballClose:  {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballClose"},
	}
}

func (at *PowerballType) GetPayload() types.Message {
	return &PowerballAction{}
}

func (powerball PowerballType) CreateTx(action string, message json.RawMessage) (*types.Transaction, error) {
	plog.Debug("powerball.CreateTx", "action", action)
	var tx *types.Transaction
	if action == "PowerballCreate" {
		var param PowerballCreateTx
		err := json.Unmarshal(message, &param)
		if err != nil {
			plog.Error("CreateTx", "Error", err)
			return nil, types.ErrInvalidParam
		}
		return CreateRawPowerballCreateTx(&param)
	} else if action == "PowerballBuy" {
		var param PowerballBuyTx
		err := json.Unmarshal(message, &param)
		if err != nil {
			plog.Error("CreateTx", "Error", err)
			return nil, types.ErrInvalidParam
		}
		return CreateRawPowerballBuyTx(&param)
	} else if action == "PowerballDraw" {
		var param PowerballDrawTx
		err := json.Unmarshal(message, &param)
		if err != nil {
			plog.Error("CreateTx", "Error", err)
			return nil, types.ErrInvalidParam
		}
		return CreateRawPowerballDrawTx(&param)
	} else if action == "PowerballClose" {
		var param PowerballCloseTx
		err := json.Unmarshal(message, &param)
		if err != nil {
			plog.Error("CreateTx", "Error", err)
			return nil, types.ErrInvalidParam
		}
		return CreateRawPowerballCloseTx(&param)
	} else {
		return nil, types.ErrNotSupport
	}

	return tx, nil
}

func (lott PowerballType) GetTypeMap() map[string]int32 {
	return map[string]int32{
		"Create": PowerballActionCreate,
		"Buy":    PowerballActionBuy,
		"Draw":   PowerballActionDraw,
		"Close":  PowerballActionClose,
	}
}

func CreateRawPowerballCreateTx(parm *PowerballCreateTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballCreateTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballCreate{
		PurBlockNum:  parm.PurBlockNum,
		DrawBlockNum: parm.DrawBlockNum,
	}
	create := &PowerballAction{
		Ty:    PowerballActionCreate,
		Value: &PowerballAction_Create{v},
	}
	tx := &types.Transaction{
		Execer:  []byte(types.ExecName(PowerballX)),
		Payload: types.Encode(create),
		Fee:     parm.Fee,
		To:      address.ExecAddress(types.ExecName(PowerballX)),
	}
	name := types.ExecName(PowerballX)
	tx, err := types.FormatTx(name, tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func CreateRawPowerballBuyTx(parm *PowerballBuyTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballBuyTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballBuy{
		PowerballId: parm.PowerballId,
		Amount:      parm.Amount,
		Number:      parm.Number,
		Way:         parm.Way,
	}
	buy := &PowerballAction{
		Ty:    PowerballActionBuy,
		Value: &PowerballAction_Buy{v},
	}
	tx := &types.Transaction{
		Execer:  []byte(types.ExecName(PowerballX)),
		Payload: types.Encode(buy),
		Fee:     parm.Fee,
		To:      address.ExecAddress(types.ExecName(PowerballX)),
	}
	name := types.ExecName(PowerballX)
	tx, err := types.FormatTx(name, tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func CreateRawPowerballDrawTx(parm *PowerballDrawTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballDrawTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballDraw{
		PowerballId: parm.PowerballId,
	}
	draw := &PowerballAction{
		Ty:    PowerballActionDraw,
		Value: &PowerballAction_Draw{v},
	}
	tx := &types.Transaction{
		Execer:  []byte(types.ExecName(PowerballX)),
		Payload: types.Encode(draw),
		Fee:     parm.Fee,
		To:      address.ExecAddress(types.ExecName(PowerballX)),
	}
	name := types.ExecName(PowerballX)
	tx, err := types.FormatTx(name, tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func CreateRawPowerballCloseTx(parm *PowerballCloseTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballCloseTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballClose{
		PowerballId: parm.PowerballId,
	}
	close := &PowerballAction{
		Ty:    PowerballActionClose,
		Value: &PowerballAction_Close{v},
	}
	tx := &types.Transaction{
		Execer:  []byte(types.ExecName(PowerballX)),
		Payload: types.Encode(close),
		Fee:     parm.Fee,
		To:      address.ExecAddress(types.ExecName(PowerballX)),
	}

	name := types.ExecName(PowerballX)
	tx, err := types.FormatTx(name, tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
