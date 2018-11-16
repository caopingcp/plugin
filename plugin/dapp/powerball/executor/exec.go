// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

func (l *Powerball) Exec_Create(payload *pty.PowerballCreate, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballCreate(payload)
}

func (l *Powerball) Exec_Buy(payload *pty.PowerballBuy, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballBuy(payload)
}

func (l *Powerball) Exec_Draw(payload *pty.PowerballDraw, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballDraw(payload)
}

func (l *Powerball) Exec_Close(payload *pty.PowerballClose, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballClose(payload)
}
