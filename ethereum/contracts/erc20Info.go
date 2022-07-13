// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// ERC20InfoMetaData contains all meta data concerning the ERC20Info contract.
var ERC20InfoMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"decimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b5061041d806100206000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c80630198489214610051578063a86e35761461007a578063d449a8321461008d578063e4dc2aa4146100ad575b600080fd5b61006461005f36600461026b565b6100cd565b604051610071919061036d565b60405180910390f35b61006461008836600461026b565b61014a565b6100a061009b36600461026b565b610185565b60405161007191906103a9565b6100c06100bb36600461026b565b6101f8565b60405161007191906103a0565b6060816001600160a01b03166306fdde036040518163ffffffff1660e01b815260040160006040518083038186803b15801561010857600080fd5b505afa15801561011c573d6000803e3d6000fd5b505050506040513d6000823e601f3d908101601f191682016040526101449190810190610299565b92915050565b6060816001600160a01b03166395d89b416040518163ffffffff1660e01b815260040160006040518083038186803b15801561010857600080fd5b6000816001600160a01b031663313ce5676040518163ffffffff1660e01b815260040160206040518083038186803b1580156101c057600080fd5b505afa1580156101d4573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610144919061034c565b6000816001600160a01b03166318160ddd6040518163ffffffff1660e01b815260040160206040518083038186803b15801561023357600080fd5b505afa158015610247573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906101449190610334565b60006020828403121561027c578081fd5b81356001600160a01b0381168114610292578182fd5b9392505050565b6000602082840312156102aa578081fd5b815167ffffffffffffffff808211156102c1578283fd5b818401915084601f8301126102d4578283fd5b8151818111156102e2578384fd5b604051601f8201601f191681016020018381118282101715610302578586fd5b604052818152838201602001871015610319578485fd5b61032a8260208301602087016103b7565b9695505050505050565b600060208284031215610345578081fd5b5051919050565b60006020828403121561035d578081fd5b815160ff81168114610292578182fd5b600060208252825180602084015261038c8160408501602087016103b7565b601f01601f19169190910160400192915050565b90815260200190565b60ff91909116815260200190565b60005b838110156103d25781810151838201526020016103ba565b838111156103e1576000848401525b5050505056fea2646970667358221220dcf97c528f4efee3987d371d5ff12c3a32f1eea6411feda2f75eae557aebddaf64736f6c634300060c0033",
}

// ERC20InfoABI is the input ABI used to generate the binding from.
// Deprecated: Use ERC20InfoMetaData.ABI instead.
var ERC20InfoABI = ERC20InfoMetaData.ABI

// ERC20InfoBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use ERC20InfoMetaData.Bin instead.
var ERC20InfoBin = ERC20InfoMetaData.Bin

// DeployERC20Info deploys a new Ethereum contract, binding an instance of ERC20Info to it.
func DeployERC20Info(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *ERC20Info, error) {
	parsed, err := ERC20InfoMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(ERC20InfoBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &ERC20Info{ERC20InfoCaller: ERC20InfoCaller{contract: contract}, ERC20InfoTransactor: ERC20InfoTransactor{contract: contract}, ERC20InfoFilterer: ERC20InfoFilterer{contract: contract}}, nil
}

// ERC20Info is an auto generated Go binding around an Ethereum contract.
type ERC20Info struct {
	ERC20InfoCaller     // Read-only binding to the contract
	ERC20InfoTransactor // Write-only binding to the contract
	ERC20InfoFilterer   // Log filterer for contract events
}

// ERC20InfoCaller is an auto generated read-only Go binding around an Ethereum contract.
type ERC20InfoCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ERC20InfoTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ERC20InfoTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ERC20InfoFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ERC20InfoFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ERC20InfoSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ERC20InfoSession struct {
	Contract     *ERC20Info        // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ERC20InfoCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ERC20InfoCallerSession struct {
	Contract *ERC20InfoCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts    // Call options to use throughout this session
}

// ERC20InfoTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ERC20InfoTransactorSession struct {
	Contract     *ERC20InfoTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// ERC20InfoRaw is an auto generated low-level Go binding around an Ethereum contract.
type ERC20InfoRaw struct {
	Contract *ERC20Info // Generic contract binding to access the raw methods on
}

// ERC20InfoCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ERC20InfoCallerRaw struct {
	Contract *ERC20InfoCaller // Generic read-only contract binding to access the raw methods on
}

// ERC20InfoTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ERC20InfoTransactorRaw struct {
	Contract *ERC20InfoTransactor // Generic write-only contract binding to access the raw methods on
}

// NewERC20Info creates a new instance of ERC20Info, bound to a specific deployed contract.
func NewERC20Info(address common.Address, backend bind.ContractBackend) (*ERC20Info, error) {
	contract, err := bindERC20Info(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ERC20Info{ERC20InfoCaller: ERC20InfoCaller{contract: contract}, ERC20InfoTransactor: ERC20InfoTransactor{contract: contract}, ERC20InfoFilterer: ERC20InfoFilterer{contract: contract}}, nil
}

// NewERC20InfoCaller creates a new read-only instance of ERC20Info, bound to a specific deployed contract.
func NewERC20InfoCaller(address common.Address, caller bind.ContractCaller) (*ERC20InfoCaller, error) {
	contract, err := bindERC20Info(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ERC20InfoCaller{contract: contract}, nil
}

// NewERC20InfoTransactor creates a new write-only instance of ERC20Info, bound to a specific deployed contract.
func NewERC20InfoTransactor(address common.Address, transactor bind.ContractTransactor) (*ERC20InfoTransactor, error) {
	contract, err := bindERC20Info(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ERC20InfoTransactor{contract: contract}, nil
}

// NewERC20InfoFilterer creates a new log filterer instance of ERC20Info, bound to a specific deployed contract.
func NewERC20InfoFilterer(address common.Address, filterer bind.ContractFilterer) (*ERC20InfoFilterer, error) {
	contract, err := bindERC20Info(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ERC20InfoFilterer{contract: contract}, nil
}

// bindERC20Info binds a generic wrapper to an already deployed contract.
func bindERC20Info(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ERC20InfoABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ERC20Info *ERC20InfoRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ERC20Info.Contract.ERC20InfoCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ERC20Info *ERC20InfoRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ERC20Info.Contract.ERC20InfoTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ERC20Info *ERC20InfoRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ERC20Info.Contract.ERC20InfoTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ERC20Info *ERC20InfoCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ERC20Info.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ERC20Info *ERC20InfoTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ERC20Info.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ERC20Info *ERC20InfoTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ERC20Info.Contract.contract.Transact(opts, method, params...)
}

// Decimals is a free data retrieval call binding the contract method 0xd449a832.
//
// Solidity: function decimals(address tokenAddress) view returns(uint8)
func (_ERC20Info *ERC20InfoCaller) Decimals(opts *bind.CallOpts, tokenAddress common.Address) (uint8, error) {
	var out []interface{}
	err := _ERC20Info.contract.Call(opts, &out, "decimals", tokenAddress)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0xd449a832.
//
// Solidity: function decimals(address tokenAddress) view returns(uint8)
func (_ERC20Info *ERC20InfoSession) Decimals(tokenAddress common.Address) (uint8, error) {
	return _ERC20Info.Contract.Decimals(&_ERC20Info.CallOpts, tokenAddress)
}

// Decimals is a free data retrieval call binding the contract method 0xd449a832.
//
// Solidity: function decimals(address tokenAddress) view returns(uint8)
func (_ERC20Info *ERC20InfoCallerSession) Decimals(tokenAddress common.Address) (uint8, error) {
	return _ERC20Info.Contract.Decimals(&_ERC20Info.CallOpts, tokenAddress)
}

// Name is a free data retrieval call binding the contract method 0x01984892.
//
// Solidity: function name(address tokenAddress) view returns(string)
func (_ERC20Info *ERC20InfoCaller) Name(opts *bind.CallOpts, tokenAddress common.Address) (string, error) {
	var out []interface{}
	err := _ERC20Info.contract.Call(opts, &out, "name", tokenAddress)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x01984892.
//
// Solidity: function name(address tokenAddress) view returns(string)
func (_ERC20Info *ERC20InfoSession) Name(tokenAddress common.Address) (string, error) {
	return _ERC20Info.Contract.Name(&_ERC20Info.CallOpts, tokenAddress)
}

// Name is a free data retrieval call binding the contract method 0x01984892.
//
// Solidity: function name(address tokenAddress) view returns(string)
func (_ERC20Info *ERC20InfoCallerSession) Name(tokenAddress common.Address) (string, error) {
	return _ERC20Info.Contract.Name(&_ERC20Info.CallOpts, tokenAddress)
}

// Symbol is a free data retrieval call binding the contract method 0xa86e3576.
//
// Solidity: function symbol(address tokenAddress) view returns(string)
func (_ERC20Info *ERC20InfoCaller) Symbol(opts *bind.CallOpts, tokenAddress common.Address) (string, error) {
	var out []interface{}
	err := _ERC20Info.contract.Call(opts, &out, "symbol", tokenAddress)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0xa86e3576.
//
// Solidity: function symbol(address tokenAddress) view returns(string)
func (_ERC20Info *ERC20InfoSession) Symbol(tokenAddress common.Address) (string, error) {
	return _ERC20Info.Contract.Symbol(&_ERC20Info.CallOpts, tokenAddress)
}

// Symbol is a free data retrieval call binding the contract method 0xa86e3576.
//
// Solidity: function symbol(address tokenAddress) view returns(string)
func (_ERC20Info *ERC20InfoCallerSession) Symbol(tokenAddress common.Address) (string, error) {
	return _ERC20Info.Contract.Symbol(&_ERC20Info.CallOpts, tokenAddress)
}

// TotalSupply is a free data retrieval call binding the contract method 0xe4dc2aa4.
//
// Solidity: function totalSupply(address tokenAddress) view returns(uint256)
func (_ERC20Info *ERC20InfoCaller) TotalSupply(opts *bind.CallOpts, tokenAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _ERC20Info.contract.Call(opts, &out, "totalSupply", tokenAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0xe4dc2aa4.
//
// Solidity: function totalSupply(address tokenAddress) view returns(uint256)
func (_ERC20Info *ERC20InfoSession) TotalSupply(tokenAddress common.Address) (*big.Int, error) {
	return _ERC20Info.Contract.TotalSupply(&_ERC20Info.CallOpts, tokenAddress)
}

// TotalSupply is a free data retrieval call binding the contract method 0xe4dc2aa4.
//
// Solidity: function totalSupply(address tokenAddress) view returns(uint256)
func (_ERC20Info *ERC20InfoCallerSession) TotalSupply(tokenAddress common.Address) (*big.Int, error) {
	return _ERC20Info.Contract.TotalSupply(&_ERC20Info.CallOpts, tokenAddress)
}
