package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"log"

	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/store"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
)

func main() {
	var dir string
	var typ string
	var height int
	flag.StringVar(&dir, "dir", "", "StateDB directory")
	flag.StringVar(&typ, "typ", "", "Database type {pebble, badger}")
	flag.IntVar(&height, "height", -1, "StateDB height")
	flag.Parse()

	if dir == "" {
		log.Fatal("Missing flag dir")
	}
	if typ == "" {
		log.Fatal("Missing flag typ")
	}
	if height == -1 {
		log.Fatal("Missing flag height")
	}

	// database, err := metadb.New(typ, dir)
	// if err != nil {
	// 	log.Fatalf("Can't open DB: %v", err)
	// }
	// sdb := statedb.NewStateDB(database)
	// root, err := sdb.VersionRoot(uint32(height))
	// if err != nil {
	// 	log.Fatalf("Can't get VersionRoot: %v", err)
	// }
	// fmt.Printf("height: %v, root: %x\n", height, root)
	// snapshot, err := sdb.TreeView(root)
	// if err != nil {
	// 	log.Fatalf("Can't get TreeView at root %x: %v", root, err)
	// }
	// fmt.Println("--- mainTree ---")
	// if err := snapshot.PrintGraphviz(); err != nil {
	// 	log.Fatalf("Can't PrintGraphviz: %v", err)
	// }
	// processes, err := snapshot.DeepSubTree(vochain.ProcessesCfg)
	// if err != nil {
	// 	log.Fatalf("Can't get Processes: %v", err)
	// }
	// fmt.Println("--- processes ---")
	// if err := processes.PrintGraphviz(); err != nil {
	// 	log.Fatalf("Can't PrintGraphviz: %v", err)
	// }
	// fmt.Println("--- processes ---")
	// processes.Iterate(func(pid, processBytes []byte) bool {
	// 	var process models.StateDBProcess
	// 	if err := proto.Unmarshal(processBytes, &process); err != nil {
	// 		log.Fatalf("Cannot unmarshal process (%s): %w", pid, err)
	// 	}
	// 	fmt.Printf("pid: %x, census: %x, votes: %x\n",
	// 		pid, process.Process.CensusRoot, process.VotesRoot)
	// 	return false
	// })
	// pid, err := hex.DecodeString(
	// 	"691fa1cf551d06d155a7c80c133c7cb9edb16f7185d079e34c3874bb53bc0e67")
	// if err != nil {
	// 	panic(err)
	// }
	// votes, err := snapshot.DeepSubTree(vochain.ProcessesCfg, vochain.VotesCfg.WithKey(pid))
	// if err != nil {
	// 	log.Fatalf("Can't get Votes: %v", err)
	// }
	// // count := 0
	// votes.Iterate(func(voteID, voteBytes []byte) bool {
	// 	// count++
	// 	fmt.Printf("%x %x\n", voteID, voteBytes)
	// 	return false
	// })
	// fmt.Printf("Votes: %v\n", count)
	cfg := tmcfg.DefaultConfig()
	cfg.RootDir = dir
	blockStoreDB, err := node.DefaultDBProvider(&node.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		log.Fatal("Can't open blockstore")
	}
	blockStore := store.NewBlockStore(blockStoreDB)
	block := blockStore.LoadBlock(int64(height))
	fmt.Printf("Block txs: %v\n", len(block.Data.Txs))
	// signedTx := new(models.SignedTx)
	// tx := new(models.Tx)
	for _, blockTx := range block.Data.Txs {
		tx, txBytes, signature, err := vochain.UnmarshalTx(blockTx)
		if err != nil {
			log.Fatalf("cannot Unmarshaltx: %v", err)
		}
		// if err = proto.Unmarshal(blockTx, signedTx); err != nil {
		// 	log.Fatalf("cannot get signed tx: %v", err)
		// }
		// if err = proto.Unmarshal(signedTx.Tx, tx); err != nil {
		// 	log.Fatalf("cannot get tx: %v", err)
		// }
		// fmt.Printf("%+v\n", tx)
		vote := tx.GetVote()
		if vote == nil {
			continue
		}
		pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
		if err != nil {
			log.Fatalf("cannot extract public key from signature: %v", err)
		}
		addr, err := ethereum.AddrFromPublicKey(pubKey)
		if err != nil {
			log.Fatalf("cannot extract address from public key: %v", err)
		}
		vote.Nullifier = vochain.GenerateNullifier(addr, vote.ProcessId)
		// fmt.Printf("%+v\n", vote)
		vid, err := voteID(vote.ProcessId, vote.Nullifier)
		if err != nil {
			log.Fatalf("cannot get voteID: %v", err)
		}
		fmt.Printf("%x\n", vid)
	}
}

func voteID(pid, nullifier []byte) ([]byte, error) {
	if len(pid) != types.ProcessIDsize {
		return nil, fmt.Errorf("wrong processID size %d", len(pid))
	}
	if len(nullifier) != types.VoteNullifierSize {
		return nil, fmt.Errorf("wrong nullifier size %d", len(nullifier))
	}
	vid := sha256.New()
	vid.Write(pid)
	vid.Write(nullifier)
	return vid.Sum(nil), nil
}
