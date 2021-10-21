package vochain

import (
	"testing"

	qt "github.com/frankban/quicktest"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

func TestBalanceTransfer(t *testing.T) {
	log.Init("info", "stdout")
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	addr1 := ethereum.SignKeys{}
	addr1.Generate()
	addr2 := ethereum.SignKeys{}
	addr2.Generate()

	err = s.MintBalance(addr1.Address(), 50)
	qt.Assert(t, err, qt.IsNil)

	s.Save() // Save to test isQuery value on next call
	b1, err := s.GetAccount(addr1.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b1.Balance, qt.Equals, uint64(50))
	qt.Assert(t, b1.Nonce, qt.Equals, uint32(0))

	err = s.TransferBalance(addr1.Address(), addr2.Address(), 20, 0, false)
	qt.Assert(t, err, qt.IsNil)

	b2, err := s.GetAccount(addr2.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Balance, qt.Equals, uint64(20))

	err = s.TransferBalance(addr1.Address(), addr2.Address(), 20, 2, false)
	qt.Assert(t, err, qt.IsNotNil)

	err = s.TransferBalance(addr1.Address(), addr2.Address(), 40, 1, false)
	qt.Assert(t, err, qt.IsNotNil)

	err = s.TransferBalance(addr2.Address(), addr1.Address(), 10, 0, false)
	qt.Assert(t, err, qt.IsNil)

	err = s.TransferBalance(addr2.Address(), addr1.Address(), 5, 1, false)
	qt.Assert(t, err, qt.IsNil)

	b1, err = s.GetAccount(addr1.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b1.Balance, qt.Equals, uint64(45))
	qt.Assert(t, b1.Nonce, qt.Equals, uint32(1))

	s.Save()
	b2, err = s.GetAccount(addr2.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Balance, qt.Equals, uint64(5))
	qt.Assert(t, b2.Nonce, qt.Equals, uint32(2))
}

func TestNewProcessTransactionCost(t *testing.T) {
	log.Init("debug", "stdout")
	app := TestBaseApplication(t)
	owner := &ethereum.SignKeys{}
	owner.Generate()

	// Give credits to the owner
	err := app.State.MintBalance(owner.Address(), NewProcessCost*2)
	qt.Assert(t, err, qt.IsNil)
	app.Commit()
	acc, err := app.State.GetAccount(owner.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc.GetBalance(), qt.Equals, uint64(NewProcessCost*2))

	// Create a first process, should work
	rng := testutil.NewRandom(0)
	censusURI := "ipfs://foobar"
	process := &models.Process{
		ProcessId:    rng.RandomBytes(types.ProcessIDsize),
		StartBlock:   app.Height() + 2,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true, DynamicCensus: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 2},
		Status:       models.ProcessStatus_READY,
		EntityId:     owner.Address().Bytes(),
		CensusRoot:   rng.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   10,
	}

	detxresp := app.DeliverTx(buildProcessTx(process, owner, t))
	qt.Assert(t, detxresp.Code, qt.Equals, uint32(0), qt.Commentf("resp: %s", detxresp.Data))
	app.Commit()

	acc, err = app.State.GetAccount(owner.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc.GetBalance(), qt.Equals, uint64(NewProcessCost))

	// Same process, should fail
	detxresp = app.DeliverTx(buildProcessTx(process, owner, t))
	qt.Assert(t, detxresp.Code, qt.Equals, uint32(1))

	// Second process, should work
	process.ProcessId = rng.RandomBytes(types.ProcessIDsize)
	detxresp = app.DeliverTx(buildProcessTx(process, owner, t))
	qt.Assert(t, detxresp.Code, qt.Equals, uint32(0), qt.Commentf("resp: %s", detxresp.Data))
	app.Commit()

	// Balance now should be zero
	acc, err = app.State.GetAccount(owner.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc.GetBalance(), qt.Equals, uint64(0))

	// Third process, should not work
	process.ProcessId = rng.RandomBytes(types.ProcessIDsize)
	detxresp = app.DeliverTx(buildProcessTx(process, owner, t))
	qt.Assert(t, detxresp.Code, qt.Equals, uint32(1))
}

func buildProcessTx(process *models.Process, owner *ethereum.SignKeys, t *testing.T) abcitypes.RequestDeliverTx {
	var err error
	rng := testutil.NewRandom(0)
	var stx models.SignedTx
	tx := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   rng.RandomBytes(32),
		Process: process,
	}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: tx}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = owner.Sign(stx.Tx)
	qt.Assert(t, err, qt.IsNil)
	var detx abcitypes.RequestDeliverTx
	detx.Tx, err = proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)
	return detx
}
