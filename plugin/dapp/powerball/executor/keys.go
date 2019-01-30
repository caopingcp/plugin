// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import "fmt"

func calcPowerballBuyPrefix(powerballID string, addr string) []byte {
	key := fmt.Sprintf("LODB-powerball-buy:%s:%s", powerballID, addr)
	return []byte(key)
}

func calcPowerballBuyRoundPrefix(powerballID string, addr string, round int64) []byte {
	key := fmt.Sprintf("LODB-powerball-buy:%s:%s:%10d", powerballID, addr, round)
	return []byte(key)
}

func calcPowerballBuyKey(powerballID string, addr string, round int64, index int64) []byte {
	key := fmt.Sprintf("LODB-powerball-buy:%s:%s:%10d:%18d", powerballID, addr, round, index)
	return []byte(key)
}

func calcPowerballDrawPrefix(powerballID string) []byte {
	key := fmt.Sprintf("LODB-powerball-draw:%s", powerballID)
	return []byte(key)
}

func calcPowerballDrawKey(powerballID string, round int64) []byte {
	key := fmt.Sprintf("LODB-powerball-draw:%s:%10d", powerballID, round)
	return []byte(key)
}

func calcPowerballKey(powerballID string, status int32) []byte {
	key := fmt.Sprintf("LODB-powerball-:%s:%d", powerballID, status)
	return []byte(key)
}

func calcPowerballGainPrefix(powerballID string, addr string) []byte {
	key := fmt.Sprintf("LODB-powerball-gain:%s:%s", powerballID, addr)
	return []byte(key)
}

func calcPowerballGainKey(powerballID string, addr string, round int64) []byte {
	key := fmt.Sprintf("LODB-powerball-gain:%s:%s:%10d", powerballID, addr, round)
	return []byte(key)
}