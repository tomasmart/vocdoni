package vochain

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// https://ipfs.io/ipfs/QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF
const infoURI string = "ipfs://QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF"
const randomEthAccount = "0x52bc44d5378309ee2abf1539bf71de1b7d7be3b5"

func TestSetAccountInfoTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}

	// should create an account
	if err := testSetAccountInfoTx(t, &signer, app, infoURI); err != nil {
		t.Fatal(err)
	}

	// should fail if infoURI is empty
	if err := testSetAccountInfoTx(t, &signer, app, ""); err == nil {
		t.Fatal(err)
	}

	// get account
	acc, err := app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	if acc.InfoURI != infoURI {
		t.Fatal(fmt.Sprintf("infoURI missmatch, got %s expected %s", acc.InfoURI, infoURI))
	}
}

func testSetAccountInfoTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	infoURI string) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	tx := &models.SetAccountInfoTx{
		Txtype:  models.TxType_SET_ACCOUNT_INFO,
		InfoURI: infoURI,
	}

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountInfo{SetAccountInfo: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
		t.Fatal(err)
	}

	if cktx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

func TestMintTokensTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}

	app.State.setTreasurer(signer.Address())
	app.Commit()

	// should mint
	if err := testMintTokensTx(t, &signer, app, randomEthAccount, 100); err != nil {
		t.Fatal(err)
	}

	// should fail minting
	if err := testMintTokensTx(t, &signer, app, randomEthAccount, 0); err == nil {
		t.Fatal(err)
	}

	// get account
	toAcc := common.HexToAddress(randomEthAccount)
	acc, err := app.State.GetAccount(toAcc, false)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Balance != 100 {
		t.Fatal(fmt.Sprintf("infoURI missmatch, got %d expected %d", acc.Balance, 100))
	}
	// get treasurer
	treasurer, err := app.State.Treasurer(false)
	if err != nil {
		t.Fatal(err)
	}
	// check nonce incremented
	if treasurer.Nonce != 1 {
		t.Fatal(err)
	}
}

func testMintTokensTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	to string,
	value uint64) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	toAddr := common.HexToAddress(to)
	tx := &models.MintTokensTx{
		Txtype: models.TxType_MINT_TOKENS,
		To:     toAddr.Bytes(),
		Value:  value,
		Nonce:  0,
	}

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_MintTokens{MintTokens: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
		t.Fatal(err)
	}

	if cktx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}