// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

type PowerballCreateTx struct {
	PurBlockNum  int64 `json:"purBlockNum"`
	DrawBlockNum int64 `json:"drawBlockNum"`
	Fee          int64 `json:"fee"`
}

type PowerballBuyTx struct {
	PowerballId string `json:"powerballId"`
	Amount      int64  `json:"amount"`
	Number      int64  `json:"number"`
	Way         int64  `json:"way"`
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
