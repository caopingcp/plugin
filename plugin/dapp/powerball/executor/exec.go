// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

// Exec_Create create game
func (l *Powerball) Exec_Create(payload *pty.PowerballCreate, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballCreate(payload)
}

// Exec_Buy buy ticket
func (l *Powerball) Exec_Buy(payload *pty.PowerballBuy, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballBuy(payload)
}

// Exec_Pause pause game
func (l *Powerball) Exec_Pause(payload *pty.PowerballPause, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballPause(payload)
}

// Exec_Draw draw game
func (l *Powerball) Exec_Draw(payload *pty.PowerballDraw, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballDraw(payload)
}

// Exec_Close close game
func (l *Powerball) Exec_Close(payload *pty.PowerballClose, tx *types.Transaction, index int) (*types.Receipt, error) {
	actiondb := NewPowerballAction(l, tx, index)
	return actiondb.PowerballClose(payload)
}
