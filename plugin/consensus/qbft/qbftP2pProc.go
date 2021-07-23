// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package qbft

import (
	"fmt"

	"github.com/33cn/chain33/types"
	ttypes "github.com/33cn/plugin/plugin/consensus/qbft/types"
	tmtypes "github.com/33cn/plugin/plugin/dapp/qbftNode/types"
	"github.com/pkg/errors"
)

const (
	moduleName = "consensu"

	qbftVoteTopic    = "QBFT-VOTE-TOPIC"
	qbftAggVoteTopic = "QBFT-AGGVOTE-TOPIC"
	qbftMsgTopic     = "QBFT-MSG-TOPIC"
)

// P2PNode
type P2pNode struct {
	state            *ConsensusState
	peerSet          map[string]*PeerConnState
	broadcastChannel chan MsgInfo
	unicastChannel   chan MsgInfo
}

// NewP2pNode method
func NewP2pNode(state *ConsensusState) *P2pNode {
	node := &P2pNode{
		state:            state,
		peerSet:          make(map[string]*PeerConnState),
		broadcastChannel: make(chan MsgInfo, maxSendQueueSize),
		unicastChannel:   make(chan MsgInfo, maxSendQueueSize),
	}
	state.SetBroadcastChannel(node.broadcastChannel)
	state.SetUnicastChannel(node.unicastChannel)
	return node
}

func (node *P2pNode) ProcMsg(from string, msg *tmtypes.QbftP2PSubMsg) {
	pc := node.peerSet[from]
	switch byte(msg.GetTy()) {
	case ttypes.ProposalID:
		proposal := msg.GetProposal()
		pc.SetHasProposal(proposal)
	case ttypes.VoteID:
		vote := &ttypes.Vote{QbftVote: msg.GetVote()}
		pc.SetHasVote(vote)
	case ttypes.ProposalBlockID:
		block := &ttypes.QbftBlock{QbftBlock: msg.GetBlock()}
		pc.SetHasProposalBlock(block)
	case ttypes.AggVoteID:
		aggVote := &ttypes.AggVote{QbftAggVote: msg.GetAggVote()}
		pc.SetHasAggVote(aggVote)
	case ttypes.NewRoundStepID:
		pc.ApplyNewRoundStepMessage(msg.GetNewRoundStep())
	case ttypes.ValidBlockID:
		pc.ApplyValidBlockMessage(msg.GetValidBlock())
	case ttypes.HasVoteID:
		pc.ApplyHasVoteMessage(msg.GetHasVote())
	case ttypes.VoteSetMaj23ID:
		maj32 := msg.GetVoteSetMaj23()
		node.state.SetPeerMaj23(maj32.Height, int(maj32.Round), byte(maj32.Type), from, maj32.BlockID)
		var myVotes *ttypes.BitArray
		switch byte(maj32.Type) {
		case ttypes.VoteTypePrevote:
			myVotes = node.state.GetPrevotesState(maj32.Height, int(maj32.Round), maj32.BlockID)
		case ttypes.VoteTypePrecommit:
			myVotes = node.state.GetPrecommitsState(maj32.Height, int(maj32.Round), maj32.BlockID)
		default:
			qbftlog.Error("Bad VoteSetBitsMessage field Type", "type", byte(maj32.Type))
			return
		}
		if myVotes != nil && myVotes.QbftBitArray != nil {
			//voteSetBitMsg := &tmtypes.QbftVoteSetBitsMsg{
			//	Height:  maj32.Height,
			//	Round:   maj32.Round,
			//	Type:    maj32.Type,
			//	BlockID: maj32.BlockID,
			//	Votes:   myVotes.QbftBitArray,
			//}
			//pc.sendQueue <- MsgInfo{TypeID: ttypes.VoteSetBitsID, Msg: voteSetBitMsg, PeerID: pc.id, PeerIP: pc.ip.String()}
		}
	case ttypes.ProposalHeartbeatID:
		heartbeat := msg.GetHeartbeat()
		qbftlog.Debug("Received proposal heartbeat message",
			"height", heartbeat.Height, "round", heartbeat.Round, "sequence", heartbeat.Sequence,
			"valIdx", heartbeat.ValidatorIndex, "valAddr", fmt.Sprintf("%X", heartbeat.ValidatorAddress))
	default:
		qbftlog.Error("paracross ProcEvent not support", "ty", msg.GetTy())
	}

}

func (client *Client) sendP2PMsg(ty int64, data interface{}) ([]byte, error) {
	msg := client.GetQueueClient().NewMessage("p2p", ty, data)
	err := client.GetQueueClient().Send(msg, true)
	if err != nil {
		return nil, errors.Wrapf(err, "ty=%d", ty)
	}
	resp, err := client.GetQueueClient().Wait(msg)
	if err != nil {
		return nil, errors.Wrapf(err, "wait ty=%d", ty)
	}

	if resp.GetData().(*types.Reply).IsOk {
		return resp.GetData().(*types.Reply).Msg, nil
	}
	return nil, errors.Wrapf(types.ErrInvalidParam, "resp msg=%s", string(resp.GetData().(*types.Reply).GetMsg()))
}

// p2p订阅消息
func (client *Client) SendP2PMsg(topic string, pid string, msg []byte) error {
	data := &types.PeerMessage{Topic: topic, Module: moduleName, PeerId: pid, Msg: msg}
	_, err := client.sendP2PMsg(types.EventPeerMessage, data)
	return err
}

func (client *Client) SendPubP2PMsg(topic string, msg []byte) error {
	data := &types.PublishTopicMsg{Topic: topic, Msg: msg}
	_, err := client.sendP2PMsg(types.EventPubTopicMsg, data)
	return err
}

func (client *Client) SendSubP2PTopic(topic string) error {
	data := &types.SubTopic{Topic: topic, Module: moduleName}
	_, err := client.sendP2PMsg(types.EventSubTopic, data)
	return err
}

func (client *Client) SendRmvP2PTopic(topic string) error {
	data := &types.RemoveTopic{Topic: topic, Module: moduleName}
	_, err := client.sendP2PMsg(types.EventRemoveTopic, data)
	return err

}

func (client *Client) SendFetchP2PTopic() (*types.TopicList, error) {
	data := &types.FetchTopicList{Module: moduleName}
	msg, err := client.sendP2PMsg(types.EventFetchTopics, data)
	if err != nil {
		return nil, errors.Wrap(err, "reply fail")
	}
	var reply types.TopicList
	err = types.Decode(msg, &reply)
	if err != nil {
		return nil, errors.Wrap(err, "decode fail")
	}
	return &reply, err
}
