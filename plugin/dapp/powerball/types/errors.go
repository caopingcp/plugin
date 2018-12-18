// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "errors"

// error info
var (
	ErrNoPrivilege                 = errors.New("ErrNoPrivilege")
	ErrPowerballStatus             = errors.New("ErrPowerballStatus")
	ErrPowerballPauseActionInvalid = errors.New("ErrPowerballPauseActionInvalid")
	ErrPowerballDrawActionInvalid  = errors.New("ErrPowerballDrawActionInvalid")
	ErrPowerballFundNotEnough      = errors.New("ErrPowerballFundNotEnough")
	ErrPowerballCreatorBuy         = errors.New("ErrPowerballCreatorBuy")
	ErrPowerballBuyAmount          = errors.New("ErrPowerballBuyAmount")
	ErrPowerballRepeatHash         = errors.New("ErrPowerballRepeatHash")
	ErrPowerballPurBlockLimit      = errors.New("ErrPowerballPurBlockLimit")
	ErrPowerballPauseBlockLimit    = errors.New("ErrPowerballPauseBlockLimit")
	ErrPowerballBuyNumber          = errors.New("ErrPowerballBuyNumber")
	ErrPowerballShowRepeated       = errors.New("ErrPowerballShowRepeated")
	ErrPowerballShowError          = errors.New("ErrPowerballShowError")
	ErrPowerballErrLuckyNum        = errors.New("ErrPowerballErrLuckyNum")
	ErrPowerballErrCloser          = errors.New("ErrPowerballErrCloser")
	ErrPowerballErrUnableClose     = errors.New("ErrPowerballErrUnableClose")
	ErrNodeNotExist                = errors.New("ErrNodeNotExist")
	ErrEmptyMinerTx                = errors.New("ErrEmptyMinerTx")
)
