// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

type PowerballCreateTx struct {
	PurTime     string `json:"purTime"`
	DrawTime    string `json:"drawTime"`
	TicketPrice int64  `json:"ticketPrice"`
	Fee         int64  `json:"fee"`
}

type PowerballBuyTx struct {
	PowerballId string   `json:"powerballId"`
	Amount      int64    `json:"amount"`
	Number      []string `json:"number"`
	Fee         int64    `json:"fee"`
}

type PowerballPauseTx struct {
	PowerballId string `json:"powerballId"`
	Fee         int64  `json:"fee"`
}

type PowerballDrawTx struct {
	PowerballId string `json:"powerballId"`
	Fee         int64  `json:"fee"`
}

type PowerballCloseTx struct {
	PowerballId string `json:"powerballId"`
	Fee         int64  `json:"fee"`
}
