// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// PowerballCreateTx struct
type PowerballCreateTx struct {
	PurTime       string `json:"purTime"`
	DrawTime      string `json:"drawTime"`
	TicketPrice   int64  `json:"ticketPrice"`
	PlatformRatio int64  `json:"platformRatio"`
	DevelopRatio  int64  `json:"developRatio"`
	Fee           int64  `json:"fee"`
}

// PowerballBuyTx struct
type PowerballBuyTx struct {
	PowerballID string `json:"powerballID"`
	Amount      int64  `json:"amount"`
	Number      string `json:"number"`
	Fee         int64  `json:"fee"`
}

// PowerballPauseTx struct
type PowerballPauseTx struct {
	PowerballID string `json:"powerballID"`
	Fee         int64  `json:"fee"`
}

// PowerballDrawTx struct
type PowerballDrawTx struct {
	PowerballID string `json:"powerballID"`
	Fee         int64  `json:"fee"`
}

// PowerballCloseTx struct
type PowerballCloseTx struct {
	PowerballID string `json:"powerballID"`
	Fee         int64  `json:"fee"`
}
