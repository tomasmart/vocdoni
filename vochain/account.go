package vochain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/tendermint/crypto"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// Account represents an amount of tokens, usually attached to an address.
// Account includes a Nonce which needs to be incremented by 1 on each transfer,
// an external URI link for metadata and a list of delegated addresses allowed
// to use the account on its behalf (in addition to himself).
type Account struct {
	models.Account
}

// Marshal encodes the Account and returns the serialized bytes.
func (a *Account) Marshal() ([]byte, error) {
	return proto.Marshal(a)
}

// Unmarshal decode a set of bytes.
func (a *Account) Unmarshal(data []byte) error {
	return proto.Unmarshal(data, a)
}

// Transfer moves amount from the origin Account to the dest Account.
func (a *Account) Transfer(dest *Account, amount uint64, cost uint64, nonce uint32) error {
	if amount == 0 {
		return fmt.Errorf("cannot transfer zero amount")
	}
	if dest == nil {
		return fmt.Errorf("destination account nil")
	}
	if a.Nonce != nonce {
		return ErrAccountNonceInvalid
	}
	a.Nonce++
	if (a.Balance + cost) < amount {
		return ErrNotEnoughBalance
	}
	if dest.Balance+amount <= dest.Balance {
		return ErrBalanceOverflow
	}
	dest.Balance += amount
	a.Balance -= amount + cost
	return nil
}

// IsDelegate checks if an address is a delegate for an account
func (a *Account) IsDelegate(addr common.Address) bool {
	for _, d := range a.DelegateAddrs {
		if bytes.Equal(addr.Bytes(), d) {
			return true
		}
	}
	return false
}

// AddDelegate adds an address to the list of delegates for an account
func (a *Account) AddDelegate(addr common.Address) error {
	if a.IsDelegate(addr) {
		return fmt.Errorf("address %s is already a delegate", addr.Hex())
	}
	a.DelegateAddrs = append(a.DelegateAddrs, addr.Bytes())
	return nil
}

// DelDelegate removes an address from the list of delegates for an account
func (a *Account) DelDelegate(addr common.Address) error {
	for i, d := range a.DelegateAddrs {
		if bytes.Equal(addr.Bytes(), d) {
			a.DelegateAddrs[i] = a.DelegateAddrs[len(a.DelegateAddrs)-1]
			a.DelegateAddrs = a.DelegateAddrs[:len(a.DelegateAddrs)-1]
			return nil
		}
	}
	return fmt.Errorf("cannot delete delegate, not found")
}

// TransferBalance transfers balance from origin address to destination address,
// and updates the state with the new values (including nonce).
// If origin address acc is not enough, ErrNotEnoughBalance is returned.
// If provided nonce does not match origin address nonce+1, ErrAccountNonceInvalid is returned.
func (v *State) TransferBalance(from, to common.Address, amount uint64, nonce uint64) error {
	var accFrom, accTo Account
	v.Tx.Lock()
	defer v.Tx.Unlock()
	accFromRaw, err := v.Tx.DeepGet(from.Bytes(), AccountsCfg)
	if err != nil {
		return err
	}
	if err := accFrom.Unmarshal(accFromRaw); err != nil {
		return err
	}
	accToRaw, err := v.Tx.DeepGet(to.Bytes(), AccountsCfg)
	if err != nil {
		return err
	}
	if err := accTo.Unmarshal(accToRaw); err != nil {
		return err
	}
	transferCost, err := v.TxCost(models.TxType_SEND_TOKENS, false)
	if err != nil {
		return err
	}
	if err := accFrom.Transfer(&accTo, amount, transferCost, uint32(nonce)); err != nil {
		return err
	}
	af, err := accFrom.Marshal()
	if err != nil {
		return err
	}
	if err := v.Tx.DeepSet(from.Bytes(), af, AccountsCfg); err != nil {
		return err
	}
	at, err := accTo.Marshal()
	if err != nil {
		return err
	}
	if err := v.Tx.DeepSet(to.Bytes(), at, AccountsCfg); err != nil {
		return err
	}
	return nil
}

// CollectFaucet transfers balance from faucet generated package address to collector address,
// and updates the state with the new values (including nonce).
func (v *State) CollectFaucet(from, to common.Address, amount uint64, nonce uint64) error {
	var accFrom, accTo Account
	v.Tx.Lock()
	defer v.Tx.Unlock()
	accFromRaw, err := v.Tx.DeepGet(from.Bytes(), AccountsCfg)
	if err != nil {
		return err
	}
	if err := accFrom.Unmarshal(accFromRaw); err != nil {
		return err
	}
	accToRaw, err := v.Tx.DeepGet(to.Bytes(), AccountsCfg)
	if err != nil {
		return err
	}
	if err := accTo.Unmarshal(accToRaw); err != nil {
		return err
	}
	if amount == 0 {
		return fmt.Errorf("cannot transfer zero amount")
	}
	transferCost, err := v.TxCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return err
	}
	if (accFrom.Balance + transferCost) < amount {
		return ErrNotEnoughBalance
	}
	if accTo.Balance+amount <= accTo.Balance {
		return ErrBalanceOverflow
	}
	accTo.Balance += amount

	accFrom.Balance -= amount + transferCost
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, nonce)
	key := from.Bytes()
	key = append(key, b...)
	if err := v.Tx.DeepSet(crypto.Sha256(key), nil, FaucetNonceCfg); err != nil {
		return err
	}
	af, err := accFrom.Marshal()
	if err != nil {
		return err
	}
	if err := v.Tx.DeepSet(from.Bytes(), af, AccountsCfg); err != nil {
		return err
	}
	at, err := accTo.Marshal()
	if err != nil {
		return err
	}
	if err := v.Tx.DeepSet(to.Bytes(), at, AccountsCfg); err != nil {
		return err
	}
	return nil
}

// MintBalance increments the existing acc of address by amount
func (v *State) MintBalance(address common.Address, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot mint a zero amount balance")
	}
	var acc Account
	v.Tx.Lock()
	defer v.Tx.Unlock()
	raw, err := v.Tx.DeepGet(address.Bytes(), AccountsCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := acc.Unmarshal(raw); err != nil {
			return err
		}
	}
	if acc.Balance+amount <= acc.Balance {
		return ErrBalanceOverflow
	}
	acc.Balance += amount
	accBytes, err := acc.Marshal()
	if err != nil {
		return err
	}
	return v.Tx.DeepSet(address.Bytes(), accBytes, AccountsCfg)
}

// GetAccount retrives the Account for an address.
// Returns a nil account and no error if the account does not exist.
func (v *State) GetAccount(address common.Address, isQuery bool) (*Account, error) {
	var acc Account
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(isQuery).DeepGet(address.Bytes(), AccountsCfg)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &acc, acc.Unmarshal(raw)
}

// VerifyAccountBalance extracts an account address from a signed message, and verifies if
// there is enough balance to cover an amount expense
func (v *State) VerifyAccountBalance(message, signature []byte, amount uint64) (bool, common.Address, error) {
	var err error
	address := common.Address{}
	address, err = ethereum.AddrFromSignature(message, signature)
	if err != nil {
		return false, address, err
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return false, address, fmt.Errorf("VerifyAccountWithAmmount: %v", err)
	}
	if acc == nil {
		return false, address, nil
	}
	return acc.Balance >= amount, address, nil
}

func (v *State) SetAccountInfoURI(address common.Address, txSender common.Address, infoURI string) error {
	if infoURI == "" {
		return fmt.Errorf("cannot set an empty URI")
	}
	var acc Account
	v.Tx.Lock()
	defer v.Tx.Unlock()
	raw, err := v.Tx.DeepGet(address.Bytes(), AccountsCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := acc.Unmarshal(raw); err != nil {
			return err
		}
	}
	acc.InfoURI = infoURI
	accBytes, err := acc.Marshal()
	if err != nil {
		return err
	}
	processCreationCost, err := v.TxCost(models.TxType_SET_PROCESS_STATUS, false)
	if err != nil {
		return err
	}
	if err := v.burnAccountBalance(txSender, processCreationCost); err != nil {
		return err
	}
	return v.Tx.DeepSet(address.Bytes(), accBytes, AccountsCfg)
}

func (v *State) CreateAccount(address common.Address, infoURI string, delegates []common.Address, initBalance uint64) error {
	// check valid address
	if address.String() == types.EthereumZeroAddress {
		return fmt.Errorf("invalid address")
	}
	// check not created
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return fmt.Errorf("cannot create account %s: %v", address.String(), err)
	}
	if acc != nil {
		return fmt.Errorf("account %s already exists", address.String())
	}
	// account not found, creating it
	// check valid infoURI, must be set on creation
	if infoURI == "" {
		return fmt.Errorf("cannot set an empty URI")
	}
	newAccount := &Account{}
	newAccount.InfoURI = infoURI
	if len(delegates) > 0 {
		newAccount.DelegateAddrs = make([][]byte, len(delegates))
		for i := 0; i < len(delegates); i++ {
			if delegates[i].String() != types.EthereumZeroAddress {
				newAccount.DelegateAddrs = append(newAccount.DelegateAddrs, delegates[i].Bytes())
			}
		}
	}
	newAccount.Balance = initBalance
	v.Tx.Lock()
	defer v.Tx.Unlock()
	accBytes, err := newAccount.Marshal()
	if err != nil {
		return err
	}
	return v.Tx.DeepSet(address.Bytes(), accBytes, AccountsCfg)
}

// SetAccountInfoTxCheck is an abstraction of ABCI checkTx for an setAccountInfoTx transaction
// If the bool returned is true means that the account does not exist and is going to be created
func SetAccountInfoTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, common.Address, bool, error) {
	tx := vtx.GetSetAccountInfo()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, common.Address{}, false, fmt.Errorf("missing signature and/or transaction")
	}
	cost, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO, false)
	if err != nil {
		return common.Address{}, common.Address{}, false, err
	}
	authorized, accountAddress, err := state.VerifyAccountBalance(txBytes, signature, cost)
	if err != nil {
		return common.Address{}, common.Address{}, false, err
	}
	if !authorized {
		return common.Address{}, common.Address{}, false, fmt.Errorf("unauthorized: %s", ErrNotEnoughBalance)
	}
	infoURI := tx.GetInfoURI()
	if infoURI == "" {
		return common.Address{}, common.Address{}, false, fmt.Errorf("invalid URI, cannot be empty")
	}
	// check account, if not exists a new one will be created
	acc, err := state.GetAccount(accountAddress, false)
	if err != nil {
		return common.Address{}, common.Address{}, false, fmt.Errorf("cannot check if account %s exists: %v", accountAddress.String(), err)
	}
	if acc == nil {
		return accountAddress, accountAddress, true, nil
	}
	return common.BytesToAddress(tx.Account), accountAddress, false, nil
}

func MintTokensTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, uint64, error) {
	tx := vtx.GetMintTokens()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, 0, fmt.Errorf("missing signature and/or transaction")
	}
	// get address from signature
	sigAddress, err := ethereum.AddrFromSignature(txBytes, signature)
	if err != nil {
		return common.Address{}, 0, err
	}
	// get treasurer
	treasurer, err := state.Treasurer(true)
	if err != nil {
		return common.Address{}, 0, err
	}
	// check signature recovered address
	tAddr := common.BytesToAddress(treasurer.Address)
	if tAddr != sigAddress {
		return common.Address{}, 0, fmt.Errorf("address recovered not treasurer: expected %s got %s", treasurer.String(), sigAddress.String())
	}
	// check nonce
	if tx.Nonce != treasurer.Nonce {
		return common.Address{}, 0, fmt.Errorf("invalid nonce %d, expected: %d", tx.Nonce, treasurer.Nonce+1)
	}
	// check to
	to := common.BytesToAddress(tx.To)
	if to.String() == types.EthereumZeroAddress || to.String() == tAddr.String() {
		return common.Address{}, 0, fmt.Errorf("invalid address to mint")
	}
	// check value
	if tx.Value <= 0 {
		return common.Address{}, 0, fmt.Errorf("invalid value")
	}
	return to, tx.Value, nil
}

func SetAccountDelegateTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, common.Address, error) {
	tx := vtx.GetSetAccountDelegateTx()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, common.Address{}, fmt.Errorf("missing signature and/or transaction")
	}
	cost, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO, false)
	if err != nil {
		return common.Address{}, common.Address{}, err
	}
	authorized, sigAddress, err := state.VerifyAccountBalance(txBytes, signature, cost)
	if err != nil {
		return common.Address{}, common.Address{}, err
	}
	if !authorized {
		return common.Address{}, common.Address{}, ErrNotEnoughBalance
	}
	// check nonce
	acc, err := state.GetAccount(sigAddress, false)
	if err != nil {
		return common.Address{}, common.Address{}, fmt.Errorf("cannot get account info: %v", err)
	}
	if tx.Nonce != acc.Nonce {
		return common.Address{}, common.Address{}, fmt.Errorf("invalid nonce, expected %d got %d", acc.Nonce, tx.Nonce)
	}
	// check delegate
	delAcc := common.BytesToAddress(tx.Delegate)
	if delAcc.String() == types.EthereumZeroAddress {
		return common.Address{}, common.Address{}, fmt.Errorf("invalid delegate address")
	}
	switch tx.Txtype {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		for i := 0; i < len(acc.DelegateAddrs); i++ {
			delegateToCmp := common.BytesToAddress(acc.DelegateAddrs[i])
			if delegateToCmp == delAcc {
				return common.Address{}, common.Address{}, fmt.Errorf("delegate already added")
			}
		}
		return sigAddress, delAcc, nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		for i := 0; i < len(acc.DelegateAddrs); i++ {
			delegateToCmp := common.BytesToAddress(acc.DelegateAddrs[i])
			if delegateToCmp == delAcc {
				return sigAddress, delAcc, nil
			}
		}
		return common.Address{}, common.Address{}, fmt.Errorf("cannot remove a non existent delegate")
	default:
		return common.Address{}, common.Address{}, fmt.Errorf("unsupported SetAccountDelegate operation")
	}
}

func (v *State) setDelegate(accountAddr, delegateAddr common.Address, txType models.TxType) error {
	// get account
	acc, err := v.GetAccount(accountAddr, false)
	if err != nil {
		return err
	}
	setDelegateCost, err := v.TxCost(models.TxType_SET_ACCOUNT_INFO, false)
	if err != nil {
		return err
	}
	switch txType {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		if err := acc.AddDelegate(delegateAddr); err != nil {
			return err
		}
		acc.Nonce++
		acc.Balance -= setDelegateCost
		return v.setAccount(accountAddr, acc)
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		if err := acc.DelDelegate(delegateAddr); err != nil {
			return err
		}
		acc.Nonce++
		acc.Balance -= setDelegateCost
		return v.setAccount(accountAddr, acc)
	default:
		return fmt.Errorf("invalid setDelegate tx type")
	}
}

type SendTokensTxCheckValues struct {
	From  common.Address
	To    common.Address
	Value uint64
	Nonce uint32
}

func SendTokensTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*SendTokensTxCheckValues, error) {
	tx := vtx.GetSendTokens()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	// get address from signature
	sigAddress, err := ethereum.AddrFromSignature(txBytes, signature)
	if err != nil {
		return nil, err
	}
	// check from
	txFromAddress := common.BytesToAddress(tx.From)
	if txFromAddress != sigAddress {
		return nil, fmt.Errorf("from (%s) field and extracted signature (%s) mismatch",
			txFromAddress.String(),
			sigAddress.String(),
		)
	}
	// check to
	txToAddress := common.BytesToAddress(tx.To)
	if txToAddress.String() == types.EthereumZeroAddress {
		return nil, fmt.Errorf("invalid address")
	}
	_, err = state.GetAccount(txToAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get to account info: %v", err)
	}
	// check nonce
	acc, err := state.GetAccount(sigAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get account info: %v", err)
	}
	if tx.Nonce != acc.Nonce {
		return nil, fmt.Errorf("invalid nonce, expected %d got %d", acc.Nonce, tx.Nonce)
	}
	cost, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO, false)
	if err != nil {
		return nil, err
	}
	// check value
	if (tx.Value + cost) > acc.Balance {
		return nil, ErrNotEnoughBalance
	}
	return &SendTokensTxCheckValues{sigAddress, txToAddress, tx.Value, tx.Nonce}, nil
}

func CollectFaucetTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, error) {
	tx := vtx.GetCollectFaucet()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, fmt.Errorf("missing signature and/or transaction")
	}
	// get recipient address from signature
	recipientAddress, err := ethereum.AddrFromSignature(txBytes, signature)
	if err != nil {
		return common.Address{}, err
	}
	// get issuer address from faucetPayload
	faucetPkgPayload := tx.FaucetPackage.GetPayload()
	faucetPackageBytes, err := proto.Marshal(faucetPkgPayload)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract faucet package payload: %v", err)
	}
	issuerAddress, err := ethereum.AddrFromSignature(faucetPackageBytes, tx.FaucetPackage.Signature)
	if err != nil {
		return common.Address{}, err
	}
	// check recipient address extracted from signature matches with payload.To
	payloadToAddr := common.BytesToAddress(faucetPkgPayload.GetTo())
	if recipientAddress != payloadToAddr {
		return common.Address{}, fmt.Errorf("address extracted from tx (%s) does not match recipient address (%s)", recipientAddress.String(), payloadToAddr.String())
	}
	// check issuer nonce not used
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPkgPayload.Identifier)
	key := issuerAddress.Bytes()
	key = append(key, b...)
	used, err := state.FaucetNonce(crypto.Sha256(key), false)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot check faucet nonce: %v", err)
	}
	if used {
		return common.Address{}, fmt.Errorf("nonce %d already used", faucetPkgPayload.Identifier)
	}
	// check issuer have enough funds
	issuerAcc, err := state.GetAccount(issuerAddress, false)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot get faucet account: %v", err)
	}
	cost, err := state.TxCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return common.Address{}, err
	}
	if (issuerAcc.Balance + cost) < faucetPkgPayload.Amount {
		return common.Address{}, fmt.Errorf("faucet does not have enough balance %d < %d", issuerAcc.Balance, faucetPkgPayload.Amount)
	}
	return issuerAddress, nil
}

func (v *State) setAccount(accountAddress common.Address, account *Account) error {
	accBytes, err := proto.Marshal(account)
	if err != nil {
		return err
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(accountAddress.Bytes(), accBytes, AccountsCfg)
}
