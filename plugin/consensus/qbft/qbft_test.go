package qbft

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/33cn/chain33/common"
	"github.com/33cn/chain33/common/address"
	"github.com/33cn/chain33/common/crypto"
	mty "github.com/33cn/chain33/system/dapp/manage/types"
	"github.com/33cn/chain33/types"
	ty "github.com/33cn/plugin/plugin/consensus/qbft/types"
	vty "github.com/33cn/plugin/plugin/dapp/qbftNode/types"
	"github.com/stretchr/testify/assert"

	//加载系统内置store, 不要依赖plugin
	_ "github.com/33cn/chain33/system"
	_ "github.com/33cn/chain33/system/dapp/init"
	_ "github.com/33cn/chain33/system/mempool/init"
	_ "github.com/33cn/chain33/system/store/init"
	"github.com/33cn/chain33/util"
	"github.com/33cn/chain33/util/testnode"
	_ "github.com/33cn/plugin/plugin/dapp/init"
	_ "github.com/33cn/plugin/plugin/store/init"
)

// 执行： go test -cover
func TestQbft(t *testing.T) {
	mock33 := testnode.New("chain33.qbft.toml", nil)
	cfg := mock33.GetClient().GetConfig()
	defer mock33.Close()
	mock33.Listen()
	t.Log(mock33.GetGenesisAddress())
	time.Sleep(time.Second)

	txs := util.GenNoneTxs(cfg, mock33.GetGenesisKey(), 10)
	for i := 0; i < len(txs); i++ {
		mock33.GetAPI().SendTx(txs[i])
	}
	mock33.WaitTx(txs[9].Hash())

	configTx := configManagerTx()
	mock33.GetAPI().SendTx(configTx)
	mock33.WaitTx(configTx.Hash())

	addTx := addNodeTx()
	mock33.GetAPI().SendTx(addTx)
	mock33.WaitTx(addTx.Hash())

	txs = util.GenNoneTxs(cfg, mock33.GetGenesisKey(), 10)
	for i := 0; i < len(txs); i++ {
		mock33.GetAPI().SendTx(txs[i])
		mock33.WaitTx(txs[i].Hash())
	}

	time.Sleep(3 * time.Second)

	var flag bool
	err := mock33.GetJSONC().Call("qbftNode.IsSync", &types.ReqNil{}, &flag)
	assert.Nil(t, err)
	assert.Equal(t, true, flag)

	var reply vty.QbftNodeInfoSet
	err = mock33.GetJSONC().Call("qbftNode.GetNodeInfo", &types.ReqNil{}, &reply)
	assert.Nil(t, err)
	assert.Len(t, reply.Nodes, 2)

	clearQbftData()
}

func addNodeTx() *types.Transaction {
	pubkey := "93E69B00BCBC817BE7E3370BA0228908C6F5E5458F781998CDD2FDF7A983EB18BCF57F838901026DC65EDAC9A1F3D251"
	nput := &vty.QbftNodeAction_Node{Node: &vty.QbftNode{PubKey: pubkey, Power: int64(2)}}
	action := &vty.QbftNodeAction{Value: nput, Ty: vty.QbftNodeActionUpdate}
	tx := &types.Transaction{Execer: []byte("qbftNode"), Payload: types.Encode(action), Fee: fee}
	tx.To = address.ExecAddress("qbftNode")
	tx.Sign(types.SECP256K1, getprivkey("CC38546E9E659D15E6B4893F0AB32A06D103931A8230B0BDE71459D2B27D6944"))
	return tx
}

func configManagerTx() *types.Transaction {
	v := &types.ModifyConfig{Key: "qbft-manager", Op: "add", Value: "14KEKbYtKKQm4wMthSK9J4La4nAiidGozt", Addr: ""}
	modify := &mty.ManageAction{
		Ty:    mty.ManageActionModifyConfig,
		Value: &mty.ManageAction_Modify{Modify: v},
	}
	tx := &types.Transaction{Execer: []byte("manage"), Payload: types.Encode(modify), Fee: fee}
	tx.To = address.ExecAddress("manage")
	tx.Sign(types.SECP256K1, getprivkey("CC38546E9E659D15E6B4893F0AB32A06D103931A8230B0BDE71459D2B27D6944"))
	return tx
}

func getprivkey(key string) crypto.PrivKey {
	cr, err := crypto.New(types.GetSignName("", types.SECP256K1))
	if err != nil {
		panic(err)
	}
	bkey, err := common.FromHex(key)
	if err != nil {
		panic(err)
	}
	priv, err := cr.PrivKeyFromBytes(bkey[:32])
	if err != nil {
		panic(err)
	}
	return priv
}

func TestQbft2(t *testing.T) {

}

func CheckState(t *testing.T, client *Client) {
	state := client.csState.GetState()
	assert.NotEmpty(t, state)
	_, curVals := state.GetValidators()
	assert.NotEmpty(t, curVals)
	assert.True(t, state.Equals(state.Copy()))

	_, vals := client.csState.GetValidators()
	assert.Len(t, vals, 1)

	storeHeight := client.csStore.LoadStateHeight()
	assert.True(t, storeHeight > 0)

	sc := client.csState.LoadCommit(storeHeight)
	assert.NotEmpty(t, sc)
	bc := client.csState.LoadCommit(storeHeight - 1)
	assert.NotEmpty(t, bc)

	assert.NotEmpty(t, client.LoadBlockState(storeHeight))
	assert.NotEmpty(t, client.LoadProposalBlock(storeHeight))

	assert.Nil(t, client.LoadBlockCommit(0))
	assert.Nil(t, client.LoadBlockState(0))
	assert.Nil(t, client.LoadProposalBlock(0))

	csdb := client.csState.blockExec.db
	assert.NotEmpty(t, csdb)
	assert.NotEmpty(t, csdb.LoadState())
	valset, err := csdb.LoadValidators(storeHeight - 1)
	assert.Nil(t, err)
	assert.NotEmpty(t, valset)

	genState, err := MakeGenesisStateFromFile("genesis.json")
	assert.Nil(t, err)
	assert.Equal(t, genState.LastBlockHeight, int64(0))

	assert.Equal(t, client.csState.Prevote(0), 1000*time.Millisecond)
	assert.Equal(t, client.csState.Precommit(0), 1000*time.Millisecond)
	assert.Equal(t, client.csState.PeerGossipSleep(), 100*time.Millisecond)
	assert.Equal(t, client.csState.PeerQueryMaj23Sleep(), 2000*time.Millisecond)
	assert.Equal(t, client.csState.IsProposer(), true)
	assert.Nil(t, client.csState.GetPrevotesState(state.LastBlockHeight, 0, nil))
	assert.Nil(t, client.csState.GetPrecommitsState(state.LastBlockHeight, 0, nil))
	assert.Len(t, client.GenesisDoc().Validators, 1)

	msg1, err := client.Query_IsHealthy(&types.ReqNil{})
	assert.Nil(t, err)
	flag := msg1.(*vty.QbftIsHealthy).IsHealthy
	assert.Equal(t, true, flag)

	msg2, err := client.Query_NodeInfo(&types.ReqNil{})
	assert.Nil(t, err)
	tvals := msg2.(*vty.QbftNodeInfoSet).Nodes
	assert.Len(t, tvals, 1)

	err = client.CommitBlock(client.GetCurrentBlock())
	assert.Nil(t, err)
}

func clearQbftData() {
	err := os.RemoveAll("datadir")
	if err != nil {
		fmt.Println("qbft data clear fail", err.Error())
	}
	fmt.Println("qbft data clear success")
}

func TestCompareHRS(t *testing.T) {
	assert.Equal(t, CompareHRS(1, 1, ty.RoundStepNewHeight, 1, 1, ty.RoundStepNewHeight), 0)

	assert.Equal(t, CompareHRS(1, 1, ty.RoundStepPrevote, 2, 1, ty.RoundStepNewHeight), -1)
	assert.Equal(t, CompareHRS(1, 1, ty.RoundStepPrevote, 1, 2, ty.RoundStepNewHeight), -1)
	assert.Equal(t, CompareHRS(1, 1, ty.RoundStepPrevote, 1, 1, ty.RoundStepPrecommit), -1)

	assert.Equal(t, CompareHRS(2, 1, ty.RoundStepNewHeight, 1, 1, ty.RoundStepPrevote), 1)
	assert.Equal(t, CompareHRS(1, 2, ty.RoundStepNewHeight, 1, 1, ty.RoundStepPrevote), 1)
	assert.Equal(t, CompareHRS(1, 1, ty.RoundStepPrecommit, 1, 1, ty.RoundStepPrevote), 1)
	fmt.Println("TestCompareHRS ok")
}
