// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "errors"

// Errors for lottery
var (
	ErrNoPrivilege   = errors.New("ErrNoPrivilege")
	ErrGuessStatus   = errors.New("ErrGuessStatus")
	ErrOverBetsLimit = errors.New("ErrOverBetsLimit")
)
