// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

const PowerballX = "powerball"

//Powerball status
const (
	PowerballCreated = 1 + iota
	PowerballPurchase
	PowerballDrawed
	PowerballClosed
)

//Powerball op
const (
	PowerballActionCreate = 1 + iota
	PowerballActionBuy
	PowerballActionDraw
	PowerballActionClose

	//log for powerball
	TyLogPowerballCreate = 801
	TyLogPowerballBuy    = 802
	TyLogPowerballDraw   = 803
	TyLogPowerballClose  = 804
)
