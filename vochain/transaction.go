package vochain

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"google.golang.org/protobuf/proto"
)

// AddTx check the validity of a transaction and adds it to the state if commit=true
func AddTx(vtx *models.Tx, state *State, commit bool) ([]byte, error) {
	if vtx == nil || state == nil || vtx.Payload == nil {
		return nil, fmt.Errorf("transaction, state or transaction payload are nil")
	}
	switch vtx.Payload.(type) {
	case *models.Tx_Vote:
		v, err := VoteTxCheck(vtx, state, commit)
		if err != nil {
			return []byte{}, fmt.Errorf("voteTxCheck %w", err)
		}
		if commit {
			return v.Nullifier, state.AddVote(v)
		}
		return v.Nullifier, nil
	case *models.Tx_Admin:
		if err := AdminTxCheck(vtx, state); err != nil {
			return []byte{}, fmt.Errorf("adminTxChek %w", err)
		}
		tx := vtx.GetAdmin()
		if commit {
			switch tx.Txtype {
			case models.TxType_ADD_ORACLE:
				return []byte{}, state.AddOracle(common.BytesToAddress(tx.Address))
			case models.TxType_REMOVE_ORACLE:
				return []byte{}, state.RemoveOracle(common.BytesToAddress(tx.Address))
			case models.TxType_ADD_VALIDATOR:
				pk, err := hexPubKeyToTendermintEd25519(fmt.Sprintf("%x", tx.PublicKey))
				if err == nil {
					if tx.Power == nil {
						return []byte{}, fmt.Errorf("power not specified on add validator transaction")
					}
					validator := &models.Validator{
						Address: pk.Address().Bytes(),
						PubKey:  pk.Bytes(),
						Power:   *tx.Power,
					}
					return []byte{}, state.AddValidator(validator)

				}
				return []byte{}, fmt.Errorf("addValidator %w", err)

			case models.TxType_REMOVE_VALIDATOR:
				return []byte{}, state.RemoveValidator(tx.Address)
			case models.TxType_ADD_PROCESS_KEYS:
				return []byte{}, state.AddProcessKeys(tx)
			case models.TxType_REVEAL_PROCESS_KEYS:
				return []byte{}, state.RevealProcessKeys(tx)
			}
		}
	case *models.Tx_CancelProcess:
		if err := CancelProcessTxCheck(vtx, state); err != nil {
			return []byte{}, fmt.Errorf("cancelProcess %w", err)
		}
		if commit {
			tx := vtx.GetCancelProcess()
			return []byte{}, state.CancelProcess(tx.ProcessId)
		}

	case *models.Tx_NewProcess:
		if p, err := NewProcessTxCheck(vtx, state); err == nil {
			if commit {
				tx := vtx.GetNewProcess()
				if tx.Process == nil {
					return []byte{}, fmt.Errorf("newprocess process is empty")
				}
				return []byte{}, state.AddProcess(p)
			}
		} else {
			return []byte{}, fmt.Errorf("newProcess %w", err)
		}
	default:
		return []byte{}, fmt.Errorf("transaction type invalid")
	}
	return []byte{}, nil
}

// UnmarshalTx splits a tx into method and args parts and does some basic checks
func UnmarshalTx(content []byte) (*models.Tx, error) {
	vtx := models.Tx{}
	return &vtx, proto.Unmarshal(content, &vtx)
}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
// All hexadecimal strings should be already sanitized (without 0x)
func VoteTxCheck(vtx *models.Tx, state *State, forCommit bool) (*models.Vote, error) {
	tx := vtx.GetVote()
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return nil, fmt.Errorf("process %x malformed", tx.ProcessId)
	}
	header := state.Header(false)
	if header == nil {
		return nil, fmt.Errorf("cannot obtain state header")
	}
	height := uint64(header.Height)
	endBlock := process.StartBlock + process.BlockCount

	if (height >= uint64(process.StartBlock) && height <= uint64(endBlock)) && process.Status == models.ProcessStatus_READY {
		// Check in case of keys required, they have been sent by some keykeeper
		if process.EnvelopeType.EncryptedVotes && process.KeyIndex != nil && *process.KeyIndex < 1 {
			return nil, fmt.Errorf("no keys available, voting is not possible")
		}

		switch {
		case process.EnvelopeType.Anonymous:
			// TODO check snark
			return nil, fmt.Errorf("snark vote not implemented")
		default: // Signature based voting
			var vote models.Vote
			vote.ProcessId = tx.ProcessId
			if vtx.Signature == nil {
				return nil, fmt.Errorf("signature missing on voteTx")
			}
			vote.VotePackage = tx.VotePackage
			if process.EnvelopeType.EncryptedVotes {
				if len(tx.EncryptionKeyIndexes) == 0 {
					return nil, fmt.Errorf("no key indexes provided on vote package")
				}
				vote.EncryptionKeyIndexes = tx.EncryptionKeyIndexes
			}

			// In order to avoid double vote check (on checkTx and deliverTx), we use a memory vote cache.
			// An element can only be added to the vote cache during checkTx.
			// Every N seconds the old votes which are not yet in the blockchain will be removed from the cache.
			// If the same vote (but different transaction) is send to the mempool, the cache will detect it and vote will be discarted.
			uid := types.UniqID(vtx, process.EnvelopeType.Anonymous)
			vp := state.VoteCacheGet(uid)

			if forCommit && vp != nil {
				// if vote is in cache, lazy check and remove it from cache
				defer state.VoteCacheDel(uid)
				if state.EnvelopeExists(vote.ProcessId, vp.Nullifier) {
					return nil, fmt.Errorf("vote already exists")
				}
			} else {
				if vp != nil {
					return nil, fmt.Errorf("vote already exist in cache")
				}
				// if not in cache, extract pubKey, generate nullifier and check merkle proof
				if tx.Proof == nil {
					return nil, fmt.Errorf("proof not found on transaction")
				}
				vp = new(types.VoteProof)
				log.Debugf("vote signature: %x", vtx.Signature)
				tx := vtx.GetVote()
				if tx == nil {
					return nil, fmt.Errorf("vote envelope transaction not found")
				}
				vp.Proof = tx.Proof
				signedBytes, err := proto.Marshal(tx)
				if err != nil {
					return nil, fmt.Errorf("cannot marshal vote transaction: %w", err)
				}
				pubk, err := ethereum.PubKeyFromSignature(signedBytes, fmt.Sprintf("%x", vtx.Signature))
				if err != nil {
					return nil, fmt.Errorf("cannot extract public key from signature: (%w)", err)
				}
				vp.PubKey, err = hex.DecodeString(pubk)
				if err != nil {
					return nil, fmt.Errorf("cannot unmarshal public key: %w", err)
				}
				addr, err := ethereum.AddrFromPublicKey(pubk)
				if err != nil {
					return nil, fmt.Errorf("cannot extract address from public key: (%w)", err)
				}
				log.Debugf("extracted public key: %x", vp.PubKey)

				// assign a nullifier
				vp.Nullifier = GenerateNullifier(addr, vote.ProcessId)
				log.Debugf("generated new vote nullifier: %x", vp.Nullifier)

				// check if vote exists
				if state.EnvelopeExists(vote.ProcessId, vp.Nullifier) {
					return nil, fmt.Errorf("vote already exists")
				}

				// check merkle proof
				vp.PubKeyDigest = snarks.Poseidon.Hash(vp.PubKey)
				if len(vp.PubKeyDigest) != 32 {
					return nil, fmt.Errorf("cannot compute Poseidon hash: (%s)", err)
				}
				valid, err := checkMerkleProof(tx.Proof, process.CensusOrigin, process.CensusMkRoot, vp.PubKeyDigest)
				if err != nil {
					return nil, fmt.Errorf("cannot check merkle proof: (%s)", err)
				}
				if !valid {
					return nil, fmt.Errorf("proof not valid")
				}
				vp.Created = time.Now()
				state.VoteCacheAdd(uid, vp)
			}
			vote.Nullifier = vp.Nullifier
			return &vote, nil
		}
	}
	return nil, fmt.Errorf("cannot add vote, invalid block frame or process canceled/paused")
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(vtx *models.Tx, state *State) (*models.Process, error) {
	tx := vtx.GetNewProcess()
	if tx.Process == nil {
		return nil, fmt.Errorf("process data is empty")
	}
	// check signature available
	if vtx.Signature == nil || tx == nil {
		return nil, fmt.Errorf("missing signature or new process transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return nil, fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	header := state.Header(false)
	if header == nil {
		return nil, fmt.Errorf("cannot fetch state header")
	}
	// start and endblock sanity check
	if int64(tx.Process.StartBlock) < header.Height {
		return nil, fmt.Errorf("cannot add process with start block lower or equal than the current tendermint height")
	}
	if tx.Process.BlockCount <= 0 {
		return nil, fmt.Errorf("cannot add process with duration lower or equal than the current tendermint height")
	}
	signedBytes, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal new process transaction")
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, signedBytes, fmt.Sprintf("%x", vtx.Signature))
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, fmt.Errorf("unauthorized to create a process, recovered addr is %s", addr.Hex())
	}
	// get process
	_, err = state.Process(tx.Process.ProcessId, false)
	if err == nil {
		return nil, fmt.Errorf("process with id (%x) already exists", tx.Process.ProcessId)
	}

	// check valid/implemented process types
	switch {
	case tx.Process.EnvelopeType.Anonymous:
		return nil, fmt.Errorf("anonymous process not yet implemented")
	case tx.Process.EnvelopeType.Serial:
		return nil, fmt.Errorf("serial process not yet implemented")
	}

	if tx.Process.EnvelopeType.EncryptedVotes || tx.Process.EnvelopeType.Anonymous {
		// We consider the zero value as nil for security
		tx.Process.EncryptionPublicKeys = make([]string, types.MaxKeyIndex)
		tx.Process.EncryptionPrivateKeys = make([]string, types.MaxKeyIndex)
		tx.Process.CommitmentKeys = make([]string, types.MaxKeyIndex)
		tx.Process.RevealKeys = make([]string, types.MaxKeyIndex)
	}
	return tx.Process, nil
}

// CancelProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func CancelProcessTxCheck(vtx *models.Tx, state *State) error {
	tx := vtx.GetCancelProcess()
	// check signature available
	if vtx.Signature == nil || tx == nil {
		return fmt.Errorf("missing signature or cancel transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	// check signature
	signedBytes, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("cannot marshal new process transaction")
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, signedBytes, fmt.Sprintf("%x", vtx.Signature))
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf("unauthorized to cancel a process, recovered addr is %s", addr.Hex())
	}
	// get process
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot cancel process %x: %s", tx.ProcessId, err)
	}
	// check process not already canceled or finalized
	if process.Status != models.ProcessStatus_READY {
		return fmt.Errorf("cannot cancel a not ready process")
	}
	endBlock := process.StartBlock + process.BlockCount
	var height int64
	if h := state.Header(false); h != nil {
		height = h.Height
	}
	if int64(endBlock) < height {
		return fmt.Errorf("cannot cancel a finalized process")
	}
	return nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(vtx *models.Tx, state *State) error {
	tx := vtx.GetAdmin()
	// check signature available
	if vtx.Signature == nil || tx == nil {
		return fmt.Errorf("missing signature or admin transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	signedBytes, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("cannot marshal new process transaction")
	}

	if authorized, addr, err := verifySignatureAgainstOracles(oracles, signedBytes, fmt.Sprintf("%x", vtx.Signature)); err != nil {
		return err
	} else if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr.Hex())
	}

	switch {
	case tx.Txtype == models.TxType_ADD_PROCESS_KEYS || tx.Txtype == models.TxType_REVEAL_PROCESS_KEYS:
		if tx.ProcessId == nil {
			return fmt.Errorf("missing processId on AdminTxCheck")
		}
		// check process exists
		process, err := state.Process(tx.ProcessId, false)
		if err != nil {
			return err
		}
		if process == nil {
			return fmt.Errorf("process with id (%x) does not exist", tx.ProcessId)
		}
		// check process actually requires keys
		if !process.EnvelopeType.EncryptedVotes && !process.EnvelopeType.Anonymous {
			return fmt.Errorf("process does not require keys")
		}
		// get the current blockchain header
		header := state.Header(false)
		if header == nil {
			return fmt.Errorf("cannot get blockchain header")
		}
		// Specific checks
		switch tx.Txtype {
		case models.TxType_ADD_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// endblock is always greater than start block so that case is also included here
			if header.Height > int64(process.StartBlock) {
				return fmt.Errorf("cannot add process keys in a started or finished process")
			}
			// process is not canceled
			if process.Status == models.ProcessStatus_CANCELED || process.Status == models.ProcessStatus_ENDED || process.Status == models.ProcessStatus_RESULTS {
				return fmt.Errorf("cannot add process keys in a canceled process")
			}
			if len(process.EncryptionPublicKeys[*tx.KeyIndex])+len(process.CommitmentKeys[*tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %s already revealed", tx.ProcessId)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return err
			}
		case models.TxType_REVEAL_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return fmt.Errorf("missing keyIndexon AdminTxCheck")
			}
			// check process is finished
			if header.Height < int64(process.StartBlock+process.BlockCount) &&
				!(process.Status == models.ProcessStatus_ENDED || process.Status == models.ProcessStatus_CANCELED) {
				return fmt.Errorf("cannot reveal keys before the process is finished")
			}
			if len(process.EncryptionPrivateKeys[*tx.KeyIndex])+len(process.RevealKeys[*tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %s already revealed", tx.ProcessId)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkAddProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.CommitmentKey == nil && tx.EncryptionPublicKey == nil) || *tx.KeyIndex < 1 || *tx.KeyIndex > types.MaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex is not already used
	if len(process.EncryptionPublicKeys[*tx.KeyIndex]) > 0 || len(process.CommitmentKeys[*tx.KeyIndex]) > 0 {
		return fmt.Errorf("key index %d alrady exist", tx.KeyIndex)
	}
	// TBD check that provided keys are correct (ed25519 for encryption and size for Commitment)
	return nil
}

func checkRevealProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.RevealKey == nil && tx.EncryptionPrivateKey == nil) || *tx.KeyIndex < 1 || *tx.KeyIndex > types.MaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex exists
	if len(process.EncryptionPublicKeys[*tx.KeyIndex]) < 1 || len(process.CommitmentKeys[*tx.KeyIndex]) < 1 {
		return fmt.Errorf("key index %d does not exist", *tx.KeyIndex)
	}
	// check keys actually work
	if tx.EncryptionPrivateKey != nil {
		if priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", tx.EncryptionPrivateKey)); err == nil {
			pub := priv.Public().Bytes()
			if fmt.Sprintf("%x", pub) != process.EncryptionPublicKeys[*tx.KeyIndex] {
				log.Debugf("%x != %s", pub, process.EncryptionPublicKeys[*tx.KeyIndex])
				return fmt.Errorf("the provided private key does not match with the stored public key on index %d", *tx.KeyIndex)
			}
		} else {
			return err
		}
	}
	if tx.RevealKey != nil {
		commitment := snarks.Poseidon.Hash(tx.RevealKey[:])
		if fmt.Sprintf("%x", commitment) != process.CommitmentKeys[*tx.KeyIndex] {
			log.Debugf("%x != %s", commitment, process.CommitmentKeys[*tx.KeyIndex])
			return fmt.Errorf("the provided commitment reveal key does not match with the stored on index %d", *tx.KeyIndex)
		}

	}
	return nil
}
