package vochain

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
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
func (a *Account) Transfer(dest *Account, amount uint64, nonce uint32) error {
	if amount == 0 {
		return fmt.Errorf("cannot transfer zero amount")
	}
	if dest == nil {
		return fmt.Errorf("destination account nil")
	}
	if a.Nonce != nonce {
		return ErrAccountNonceInvalid
	}
	if a.Balance < amount {
		return ErrNotEnoughBalance
	}
	if dest.Balance+amount <= dest.Balance {
		return ErrBalanceOverflow
	}
	dest.Balance += amount
	a.Balance -= amount
	a.Nonce++
	return nil
}

// Sub removes amount from the Account balance.
func (a *Account) Sub(amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot burn zero amount")
	}
	if a.Balance < amount {
		return ErrNotEnoughBalance
	}
	a.Balance -= amount
	a.Nonce++
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
func (a *Account) DelDelegate(addr common.Address) {
	for i, d := range a.DelegateAddrs {
		if bytes.Equal(addr.Bytes(), d) {
			a.DelegateAddrs[i] = a.DelegateAddrs[len(a.DelegateAddrs)-1]
			a.DelegateAddrs = a.DelegateAddrs[:len(a.DelegateAddrs)-1]
		}
	}
}

// TransferBalance transfers balance from origin address to destination address,
// and updates the state with the new values (including nonce).
// If origin address acc is not enough, ErrNotEnoughBalance is returned.
// If provided nonce does not match origin address nonce+1, ErrAccountNonceInvalid is returned.
// If isQuery is set to true, this method will only check against the current state (no changes will be stored)
func (v *State) TransferBalance(from, to common.Address, amount uint64, nonce uint32, isQuery bool) error {
	var accFrom, accTo Account
	if !isQuery {
		v.Tx.Lock()
		defer v.Tx.Unlock()
	}
	accFromRaw, err := v.Tx.DeepGet(from.Bytes(), AccountsCfg)
	if err != nil {
		return err
	}
	if err := accFrom.Unmarshal(accFromRaw); err != nil {
		return err
	}
	accToRaw, err := v.Tx.DeepGet(to.Bytes(), AccountsCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := accTo.Unmarshal(accToRaw); err != nil {
			return err
		}
	}
	if err := accFrom.Transfer(&accTo, amount, nonce); err != nil {
		return err
	}
	if !isQuery {
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
	}
	return nil
}

// SetAccount stores an account for an address.
// If account already exist it is overwritten.
func (v *State) SetAccount(address common.Address, account *Account) error {
	if account == nil {
		return fmt.Errorf("could not save nil account")
	}
	accBytes, err := account.Marshal()
	if err != nil {
		return err
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(address.Bytes(), accBytes, AccountsCfg)
}

// Burn transfers amount balance from address to the burn wallet.
func (v *State) Burn(address common.Address, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot burn zero amount")
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return err
	}
	return v.TransferBalance(address, common.HexToAddress("0xffffffffffffffffffff"), amount, acc.GetNonce(), false)
}

// MintBalance increments the existing acc of address by amount.
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
		return false, address, fmt.Errorf("verifyAccountWithAmmount: %w", err)
	}
	if acc == nil {
		return false, address, nil
	}
	return acc.Balance >= amount, address, nil
}

// ChargeForTx extracts balance from the address account depending on the transaction type.
// If the address is an Oracle, this function does nothing.
func (v *State) ChargeForTx(address common.Address, txType models.TxType) error {
	log.Debugf("charging %s for tx type %s", address.Hex(), txType.String())
	// Check if the address is an oracle, in that case we don't burn any balance
	if oracle, err := v.IsOracle(address); err != nil {
		return fmt.Errorf("chargeForTx: %w", err)
	} else if oracle {
		return nil
	}
	switch txType {
	case models.TxType_NEW_PROCESS:
		if err := v.Burn(address, NewProcessCost); err != nil {
			return fmt.Errorf("chargeForTx: %w", err)
		}
	case models.TxType_SET_PROCESS_CENSUS,
		models.TxType_SET_PROCESS_QUESTION_INDEX,
		models.TxType_SET_PROCESS_STATUS:
		if err := v.Burn(address, SetProcessCost); err != nil {
			return fmt.Errorf("chargeForTx: %w", err)
		}
	default:
		return fmt.Errorf("chargeForTx: txType not recognized")
	}
	return nil
}
