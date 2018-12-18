// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"reflect"
	"strings"

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

// PowerballType struct
type PowerballType struct {
	types.ExecTypeBase
}

// NewType method
func NewType() *PowerballType {
	c := &PowerballType{}
	c.SetChild(c)
	return c
}

// GetLogMap method
func (powerball *PowerballType) GetLogMap() map[int64]*types.LogInfo {
	return map[int64]*types.LogInfo{
		TyLogPowerballCreate: {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballCreate"},
		TyLogPowerballBuy:    {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballBuy"},
		TyLogPowerballDraw:   {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballDraw"},
		TyLogPowerballClose:  {reflect.TypeOf(ReceiptPowerball{}), "LogPowerballClose"},
	}
}

// GetPayload method
func (powerball *PowerballType) GetPayload() types.Message {
	return &PowerballAction{}
}

// CreateTx method
func (powerball PowerballType) CreateTx(action string, message json.RawMessage) (*types.Transaction, error) {
	plog.Debug("powerball.CreateTx", "action", action)
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
	} else if action == "PowerballPause" {
		var param PowerballPauseTx
		err := json.Unmarshal(message, &param)
		if err != nil {
			plog.Error("CreateTx", "Error", err)
			return nil, types.ErrInvalidParam
		}
		return CreateRawPowerballPauseTx(&param)
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
	}
	return nil, types.ErrNotSupport

}

// GetTypeMap method
func (powerball PowerballType) GetTypeMap() map[string]int32 {
	return map[string]int32{
		"Create": PowerballActionCreate,
		"Buy":    PowerballActionBuy,
		"Pause":  PowerballActionPause,
		"Draw":   PowerballActionDraw,
		"Close":  PowerballActionClose,
	}
}

// CreateRawPowerballCreateTx method
func CreateRawPowerballCreateTx(parm *PowerballCreateTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballCreateTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballCreate{
		PurTime:       parm.PurTime,
		DrawTime:      parm.DrawTime,
		TicketPrice:   parm.TicketPrice,
		PlatformRatio: parm.PlatformRatio,
		DevelopRatio:  parm.DevelopRatio,
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

// CreateRawPowerballBuyTx method
func CreateRawPowerballBuyTx(parm *PowerballBuyTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballBuyTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}
	balls := strings.Split(parm.Number, ",")
	if len(balls) != RedBalls+BlueBalls {
		plog.Error("CreateRawPowerballBuyTx", "len", len(balls))
		return nil, types.ErrInvalidParam
	}
	for i := 0; i < RedBalls-1; i++ {
		for j := i + 1; j < RedBalls; j++ {
			if balls[i] == balls[j] {
				plog.Error("CreateRawPowerballBuyTx same red ball", "ballNumber", balls[i])
				return nil, types.ErrInvalidParam
			}
		}
	}

	v := &PowerballBuy{
		PowerballID: parm.PowerballID,
		Amount:      parm.Amount,
		Number:      &BallNumber{balls},
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

// CreateRawPowerballPauseTx method
func CreateRawPowerballPauseTx(parm *PowerballPauseTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballPauseTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballPause{
		PowerballID: parm.PowerballID,
	}
	pause := &PowerballAction{
		Ty:    PowerballActionPause,
		Value: &PowerballAction_Pause{v},
	}
	tx := &types.Transaction{
		Execer:  []byte(types.ExecName(PowerballX)),
		Payload: types.Encode(pause),
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

// CreateRawPowerballDrawTx method
func CreateRawPowerballDrawTx(parm *PowerballDrawTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballDrawTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballDraw{
		PowerballID: parm.PowerballID,
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

// CreateRawPowerballCloseTx method
func CreateRawPowerballCloseTx(parm *PowerballCloseTx) (*types.Transaction, error) {
	if parm == nil {
		plog.Error("CreateRawPowerballCloseTx", "parm", parm)
		return nil, types.ErrInvalidParam
	}

	v := &PowerballClose{
		PowerballID: parm.PowerballID,
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
