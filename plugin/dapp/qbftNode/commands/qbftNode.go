// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/33cn/chain33/common/crypto"
	"github.com/33cn/chain33/rpc/jsonclient"
	rpctypes "github.com/33cn/chain33/rpc/types"
	"github.com/33cn/chain33/types"
	ttypes "github.com/33cn/plugin/plugin/consensus/qbft/types"
	vt "github.com/33cn/plugin/plugin/dapp/qbftNode/types"
	"github.com/spf13/cobra"
)

var (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
	genFile  = "genesis_file.json"
	pvFile   = "priv_validator_"
	//AuthBLS ...
	AuthBLS = 259
)

// ValCmd qbftNode cmd register
func ValCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "qbft",
		Short: "Construct qbft transactions",
		Args:  cobra.MinimumNArgs(1),
	}
	cmd.AddCommand(
		IsSyncCmd(),
		GetBlockInfoCmd(),
		GetNodeInfoCmd(),
		GetPerfStatCmd(),
		AddNodeCmd(),
		CreateCmd(),
	)
	return cmd
}

// IsSyncCmd query qbft is sync
func IsSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "is_sync",
		Short: "Query qbft consensus is sync",
		Run:   isSync,
	}
	return cmd
}

func isSync(cmd *cobra.Command, args []string) {
	rpcLaddr, _ := cmd.Flags().GetString("rpc_laddr")
	var res bool
	ctx := jsonclient.NewRPCCtx(rpcLaddr, "qbftNode.IsSync", nil, &res)
	ctx.Run()
}

// GetNodeInfoCmd get validator nodes
func GetNodeInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Get qbft validator nodes",
		Run:   getNodeInfo,
	}
	return cmd
}

func getNodeInfo(cmd *cobra.Command, args []string) {
	rpcLaddr, _ := cmd.Flags().GetString("rpc_laddr")
	var res *vt.QbftNodeInfoSet
	ctx := jsonclient.NewRPCCtx(rpcLaddr, "qbftNode.GetNodeInfo", nil, &res)
	ctx.Run()
}

// GetBlockInfoCmd get block info
func GetBlockInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get qbft consensus info",
		Run:   getBlockInfo,
	}
	addGetBlockInfoFlags(cmd)
	return cmd
}

func addGetBlockInfoFlags(cmd *cobra.Command) {
	cmd.Flags().Int64P("height", "t", 0, "block height (larger than 0)")
	cmd.MarkFlagRequired("height")
}

func getBlockInfo(cmd *cobra.Command, args []string) {
	rpcLaddr, _ := cmd.Flags().GetString("rpc_laddr")
	height, _ := cmd.Flags().GetInt64("height")
	req := &vt.ReqQbftBlockInfo{
		Height: height,
	}
	params := rpctypes.Query4Jrpc{
		Execer:   vt.QbftNodeX,
		FuncName: "GetBlockInfoByHeight",
		Payload:  types.MustPBToJSON(req),
	}

	var res vt.QbftBlockInfo
	ctx := jsonclient.NewRPCCtx(rpcLaddr, "Chain33.Query", params, &res)
	ctx.Run()
}

// GetPerfStatCmd get block info
func GetPerfStatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stat",
		Short: "Get qbft performance statistics",
		Run:   getPerfStat,
	}
	addGetPerfStatFlags(cmd)
	return cmd
}

func addGetPerfStatFlags(cmd *cobra.Command) {
	cmd.Flags().Int64P("start", "s", 0, "start block height")
	cmd.Flags().Int64P("end", "e", 0, "end block height")
}

func getPerfStat(cmd *cobra.Command, args []string) {
	rpcLaddr, _ := cmd.Flags().GetString("rpc_laddr")
	start, _ := cmd.Flags().GetInt64("start")
	end, _ := cmd.Flags().GetInt64("end")
	req := &vt.ReqQbftPerfStat{
		Start: start,
		End:   end,
	}
	params := rpctypes.Query4Jrpc{
		Execer:   vt.QbftNodeX,
		FuncName: "GetPerfState",
		Payload:  types.MustPBToJSON(req),
	}

	var res vt.QbftPerfStat
	ctx := jsonclient.NewRPCCtx(rpcLaddr, "Chain33.Query", params, &res)
	ctx.Run()
}

// AddNodeCmd add validator node
func AddNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add qbft validator node",
		Run:   addNode,
	}
	addNodeFlags(cmd)
	return cmd
}

func addNodeFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("pubkey", "p", "", "public key")
	cmd.MarkFlagRequired("pubkey")
	cmd.Flags().Int64P("power", "w", 0, "voting power")
	cmd.MarkFlagRequired("power")
}

func addNode(cmd *cobra.Command, args []string) {
	title, _ := cmd.Flags().GetString("title")
	cfg := types.GetCliSysParam(title)
	pubkey, _ := cmd.Flags().GetString("pubkey")
	power, _ := cmd.Flags().GetInt64("power")

	pubkeybyte, err := hex.DecodeString(pubkey)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	value := &vt.QbftNodeAction_Node{Node: &vt.QbftNode{PubKey: pubkeybyte, Power: power}}
	action := &vt.QbftNodeAction{Value: value, Ty: vt.QbftNodeActionUpdate}
	tx := &types.Transaction{Payload: types.Encode(action)}
	tx, err = types.FormatTx(cfg, vt.QbftNodeX, tx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	txHex := types.Encode(tx)
	fmt.Println(hex.EncodeToString(txHex))
}

//CreateCmd to create keyfiles
func CreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init_keyfile",
		Short: "Initialize Qbft Keyfile",
		Run:   createFiles,
	}
	addCreateCmdFlags(cmd)
	return cmd
}

func addCreateCmdFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("num", "n", "", "num of the keyfile to create")
	cmd.MarkFlagRequired("num")
	cmd.Flags().StringP("type", "t", "ed25519", "sign type of the keyfile (secp256k1, ed25519, sm2, bls)")
}

// RandStr ...
func RandStr(length int) string {
	chars := []byte{}
MAIN_LOOP:
	for {
		val := rand.Int63()
		for i := 0; i < 10; i++ {
			v := int(val & 0x3f) // rightmost 6 bits
			if v >= 62 {         // only 62 characters in strChars
				val >>= 6
				continue
			} else {
				chars = append(chars, strChars[v])
				if len(chars) == length {
					break MAIN_LOOP
				}
				val >>= 6
			}
		}
	}

	return string(chars)
}

func initCryptoImpl(signType int) error {
	ttypes.CryptoName = types.GetSignName("", signType)
	cr, err := crypto.New(ttypes.CryptoName)
	if err != nil {
		fmt.Printf("Init crypto fail: %v", err)
		return err
	}
	ttypes.ConsensusCrypto = cr
	return nil
}

func createFiles(cmd *cobra.Command, args []string) {
	// init crypto instance
	ty, _ := cmd.Flags().GetString("type")
	signType, ok := ttypes.SignMap[ty]
	if !ok {
		fmt.Println("type parameter is not valid")
		return
	}
	err := initCryptoImpl(signType)
	if err != nil {
		return
	}

	// genesis file
	genDoc := ttypes.GenesisDoc{
		ChainID:     fmt.Sprintf("chain33-%v", RandStr(6)),
		GenesisTime: time.Now(),
	}

	num, _ := cmd.Flags().GetString("num")
	n, err := strconv.Atoi(num)
	if err != nil {
		fmt.Println("num parameter is not valid digit")
		return
	}
	for i := 0; i < n; i++ {
		// create private validator file
		pvFileName := pvFile + strconv.Itoa(i) + ".json"
		privValidator := ttypes.LoadOrGenPrivValidatorFS(pvFileName)
		if privValidator == nil {
			fmt.Println("Create priv_validator file failed.")
			break
		}

		// create genesis validator by the pubkey of private validator
		gv := ttypes.GenesisValidator{
			PubKey: ttypes.KeyText{Kind: ttypes.CryptoName, Data: privValidator.GetPubKey().KeyString()},
			Power:  10,
		}
		genDoc.Validators = append(genDoc.Validators, gv)
	}

	if err := genDoc.SaveAs(genFile); err != nil {
		fmt.Println("Generated genesis file failed.")
		return
	}
	fmt.Printf("Generated genesis file path %v\n", genFile)
}
