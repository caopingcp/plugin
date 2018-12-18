// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// PowerballX name
const PowerballX = "powerball"

// ball number and range
const (
	RedBalls = 6
	RedRange = 33

	BlueBalls = 1
	BlueRange = 16
)

//Powerball status
const (
	PowerballNil = iota
	PowerballCreated
	PowerballPurchase
	PowerballPaused
	PowerballDrawed
	PowerballClosed
)

//Powerball op
const (
	PowerballActionCreate = 1 + iota
	PowerballActionBuy
	PowerballActionPause
	PowerballActionDraw
	PowerballActionClose

	//log for powerball
	TyLogPowerballCreate = 801
	TyLogPowerballBuy    = 802
	TyLogPowerballPause  = 803
	TyLogPowerballDraw   = 804
	TyLogPowerballClose  = 805
)
