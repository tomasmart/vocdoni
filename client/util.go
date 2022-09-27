package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

const (
	TimeBetweenBlocks = 6 * time.Second
	waitTimeout       = 3 * TimeBetweenBlocks
	pollInterval      = TimeBetweenBlocks / 6
)

func (c *Client) WaitUntilBlock(block uint32) {
	log.Infof("waiting for block %d...", block)
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	for {
		<-poll.C
		cb, err := c.GetCurrentBlock()
		if err != nil {
			log.Error(err)
			continue
		}
		if cb >= block {
			return
		}
		log.Debugf("current block: %d", cb)
	}
}

func (c *Client) WaitUntilNBlocks(n uint32) {
	for {
		cb, err := c.GetCurrentBlock()
		if err != nil {
			log.Error(err)
			time.Sleep(pollInterval)
			continue
		}
		c.WaitUntilBlock(cb + n)
		return
	}
}

func (c *Client) WaitUntilNextBlock() {
	c.WaitUntilNBlocks(1)
}

func (c *Client) WaitUntilTxMined(txhash types.HexBytes) error {
	log.Infof("waiting for tx %x...", txhash)
	timeout := time.After(waitTimeout)
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	for {
		select {
		case <-poll.C:
			tx, err := c.GetTxByHash(txhash)
			if err == nil {
				log.Infof("found tx %x in block %d", txhash, tx.BlockHeight)
				return nil
			}
		case <-timeout:
			return fmt.Errorf("WaitUntilTxMined(%x) timed out after %s", txhash, waitTimeout)
		}
	}
}

func (c *Client) WaitUntilProcessAvailable(pid []byte) (proc *indexertypes.Process, err error) {
	log.Infof("waiting for process %x to be registered...", pid)
	timeout := time.After(waitTimeout)
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	for {
		select {
		case <-poll.C:
			proc, err = c.GetProcessInfo(pid)
			if err == nil {
				log.Infof("found process %x", pid)
				return proc, nil
			}
		case <-timeout:
			return nil, fmt.Errorf("WaitUntilProcessAvailable(%x) timed out after %s (%w)",
				pid, waitTimeout, err)
		}
	}
}

func CreateEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = ethereum.NewSignKeys()
		if err := s[i].Generate(); err != nil {
			log.Fatal(err)
		}
	}
	return s
}

type keysBatch struct {
	Keys      []signKey      `json:"keys"`
	CensusID  types.HexBytes `json:"censusId"`
	CensusURI string         `json:"censusUri"`
}

type signKey struct {
	PrivKey string `json:"privKey"`
	PubKey  string `json:"pubKey"`
	Proof   []byte `json:"proof"`
	Value   []byte `json:"value"`
}

func SaveKeysBatch(filepath string, censusID []byte, censusURI string, keys []*ethereum.SignKeys, proofs []*Proof) error {
	if proofs != nil && (len(proofs) != len(keys)) {
		return fmt.Errorf("length of Proof are Signers are different length")
	}
	var kb keysBatch
	for i, k := range keys {
		pub, priv := k.HexString()
		if proofs != nil {
			kb.Keys = append(kb.Keys, signKey{PrivKey: priv, PubKey: pub, Proof: proofs[i].Siblings, Value: proofs[i].Value})
		} else {
			kb.Keys = append(kb.Keys, signKey{PrivKey: priv, PubKey: pub})
		}
	}
	kb.CensusID = censusID
	kb.CensusURI = censusURI
	j, err := json.Marshal(kb)
	if err != nil {
		return err
	}
	log.Infof("saved census cache file has %d bytes, got %d keys", len(j), len(keys))
	return os.WriteFile(filepath, j, 0o644)
}

func LoadKeysBatch(filepath string) ([]*ethereum.SignKeys, []*Proof, []byte, string, error) {
	jb, err := os.ReadFile(filepath)
	if err != nil {
		return nil, nil, nil, "", err
	}

	var kb keysBatch
	if err = json.Unmarshal(jb, &kb); err != nil {
		return nil, nil, nil, "", err
	}

	if len(kb.Keys) == 0 || len(kb.CensusID) == 0 || kb.CensusURI == "" {
		return nil, nil, nil, "", fmt.Errorf("keybatch file is empty or missing data")
	}

	keys := make([]*ethereum.SignKeys, len(kb.Keys))
	proofs := []*Proof{}
	for i, k := range kb.Keys {
		s := ethereum.NewSignKeys()
		if err = s.AddHexKey(k.PrivKey); err != nil {
			return nil, nil, nil, "", err
		}
		proofs = append(proofs, &Proof{Siblings: k.Proof, Value: k.Value})
		keys[i] = s
	}
	return keys, proofs, kb.CensusID, kb.CensusURI, nil
}

func Random(n int) []byte {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return bytes
}

func RandomHex(n int) string {
	return hex.EncodeToString(Random(n))
}

func genVote(encrypted bool, keys []string) ([]byte, error) {
	vp := &vochain.VotePackage{
		Votes: []int{1, 2, 3, 4, 5, 6},
	}
	var vpBytes []byte
	var err error
	if encrypted {
		first := true
		for i, k := range keys {
			if len(k) > 0 {
				log.Debugf("encrypting with key %s", k)
				pub, err := nacl.DecodePublic(k)
				if err != nil {
					return nil, fmt.Errorf("cannot decode encryption key with index %d: (%s)", i, err)
				}
				if first {
					vp.Nonce = RandomHex(rand.Intn(16) + 16)
					vpBytes, err = json.Marshal(vp)
					if err != nil {
						return nil, err
					}
					first = false
				}
				if vpBytes, err = nacl.Anonymous.Encrypt(vpBytes, pub); err != nil {
					return nil, fmt.Errorf("cannot encrypt: (%s)", err)
				}
			}
		}
	} else {
		vpBytes, err = json.Marshal(vp)
		if err != nil {
			return nil, err
		}

	}
	return vpBytes, nil
}
