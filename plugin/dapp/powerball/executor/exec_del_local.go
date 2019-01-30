// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	//"github.com/33cn/chain33/common"
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

func (l *Powerball) execDelLocal(tx *types.Transaction, receiptData *types.ReceiptData) (*types.LocalDBSet, error) {
	set := &types.LocalDBSet{}
	if receiptData.GetTy() != types.ExecOk {
		return set, nil
	}
	for _, item := range receiptData.Logs {
		switch item.Ty {
		case pty.TyLogPowerballCreate, pty.TyLogPowerballBuy, pty.TyLogPowerballPause, pty.TyLogPowerballDraw, pty.TyLogPowerballClose:
			var powerlog pty.ReceiptPowerball
			err := types.Decode(item.Log, &powerlog)
			if err != nil {
				return nil, err
			}
			kv := l.deletePowerball(&powerlog)
			set.KV = append(set.KV, kv...)

			if item.Ty == pty.TyLogPowerballBuy {
				kv := l.deletePowerballBuy(&powerlog)
				set.KV = append(set.KV, kv...)
			} else if item.Ty == pty.TyLogPowerballDraw {
				kv := l.deletePowerballDraw(&powerlog)
				set.KV = append(set.KV, kv...)
				kv = l.updatePowerballBuy(&powerlog, false)
				set.KV = append(set.KV, kv...)
				kv = l.deletePowerballGain(&powerlog)
				set.KV = append(set.KV, kv...)
			}
		}
	}
	return set, nil

}

// ExecDelLocal_Create action
func (l *Powerball) ExecDelLocal_Create(payload *pty.PowerballCreate, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return nil, nil
}

// ExecDelLocal_Buy action
func (l *Powerball) ExecDelLocal_Buy(payload *pty.PowerballBuy, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return nil, nil
}

// ExecDelLocal_Pause action
func (l *Powerball) ExecDelLocal_Pause(payload *pty.PowerballPause, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return nil, nil
}

// ExecDelLocal_Draw action
func (l *Powerball) ExecDelLocal_Draw(payload *pty.PowerballDraw, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return nil, nil
}

// ExecDelLocal_Close action
func (l *Powerball) ExecDelLocal_Close(payload *pty.PowerballClose, tx *types.Transaction, receiptData *types.ReceiptData, index int) (*types.LocalDBSet, error) {
	return nil, nil
}
