// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	//"github.com/33cn/chain33/common"
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

func (l *Powerball) execLocal(tx *types.Transaction, receipt *types.ReceiptData) (*types.LocalDBSet, error) {
	set := &types.LocalDBSet{}
	if receipt.GetTy() != types.ExecOk {
		return set, nil
	}
	for _, item := range receipt.Logs {
		switch item.Ty {
		case pty.TyLogPowerballCreate, pty.TyLogPowerballBuy, pty.TyLogPowerballDraw, pty.TyLogPowerballClose:
			var powerballlog pty.ReceiptPowerball
			err := types.Decode(item.Log, &powerballlog)
			if err != nil {
				return nil, err
			}
			kv := l.savePowerball(&powerballlog)
			set.KV = append(set.KV, kv...)

			if item.Ty == pty.TyLogPowerballBuy {
				kv := l.savePowerballBuy(&powerballlog)
				set.KV = append(set.KV, kv...)
			} else if item.Ty == pty.TyLogPowerballDraw {
				kv := l.savePowerballDraw(&powerballlog)
				set.KV = append(set.KV, kv...)
				kv = l.updatePowerballBuy(&powerballlog, true)
				set.KV = append(set.KV, kv...)
			}
		}
	}
	return set, nil
}

func (l *Powerball) ExecLocal_Create(payload *pty.PowerballCreate, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return l.execLocal(tx, receiptData)
}

func (l *Powerball) ExecLocal_Buy(payload *pty.PowerballBuy, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return l.execLocal(tx, receiptData)
}

func (l *Powerball) ExecLocal_Draw(payload *pty.PowerballDraw, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return l.execLocal(tx, receiptData)
}

func (l *Powerball) ExecLocal_Close(payload *pty.PowerballClose, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return l.execLocal(tx, receiptData)
}
