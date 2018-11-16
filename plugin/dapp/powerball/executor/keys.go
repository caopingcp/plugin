// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import "fmt"

func calcPowerballBuyPrefix(powerballId string, addr string) []byte {
	key := fmt.Sprintf("LODB-powerball-buy:%s:%s", powerballId, addr)
	return []byte(key)
}

func calcPowerballBuyRoundPrefix(powerballId string, addr string, round int64) []byte {
	key := fmt.Sprintf("LODB-powerball-buy:%s:%s:%10d", powerballId, addr, round)
	return []byte(key)
}

func calcPowerballBuyKey(powerballId string, addr string, round int64, index int64) []byte {
	key := fmt.Sprintf("LODB-powerball-buy:%s:%s:%10d:%18d", powerballId, addr, round, index)
	return []byte(key)
}

func calcPowerballDrawPrefix(powerballId string) []byte {
	key := fmt.Sprintf("LODB-powerball-draw:%s", powerballId)
	return []byte(key)
}

func calcPowerballDrawKey(powerballId string, round int64) []byte {
	key := fmt.Sprintf("LODB-powerball-draw:%s:%10d", powerballId, round)
	return []byte(key)
}

func calcPowerballKey(powerballId string, status int32) []byte {
	key := fmt.Sprintf("LODB-powerball-:%d:%s", status, powerballId)
	return []byte(key)
}
