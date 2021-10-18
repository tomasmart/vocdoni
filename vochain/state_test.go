package vochain

import (
	"fmt"
	"math/rand"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	models "go.vocdoni.io/proto/build/go/models"
)

type Random struct {
	rand *rand.Rand
}

func newRandom(seed int64) Random {
	return Random{
		rand: rand.New(rand.NewSource(seed)),
	}
}

func (r *Random) RandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := r.rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func TestStateReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	hash1Before, err := s.Save()
	qt.Assert(t, err, qt.IsNil)

	s.Close()

	s, err = NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	hash1After, err := s.Store.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, hash1After, qt.DeepEquals, hash1Before)

	s.Close()
}

func TestStateBasic(t *testing.T) {
	rng := newRandom(0)
	log.Init("info", "stdout")
	s, err := NewState(db.TypePebble, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var pids [][]byte
	for i := 0; i < 100; i++ {
		pids = append(pids, rng.RandomBytes(32))
		censusURI := "ipfs://foobar"
		p := &models.Process{EntityId: rng.RandomBytes(32), CensusURI: &censusURI, ProcessId: pids[i]}
		if err := s.AddProcess(p); err != nil {
			t.Fatal(err)
		}

		for j := 0; j < 10; j++ {
			v := &models.Vote{
				ProcessId:   pids[i],
				Nullifier:   rng.RandomBytes(32),
				VotePackage: []byte(fmt.Sprintf("%d%d", i, j)),
			}
			if err := s.AddVote(v); err != nil {
				t.Error(err)
			}
		}
		totalVotes, err := s.VoteCount(false)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, totalVotes, qt.Equals, uint64(10*(i+1)))
	}
	s.Save()

	p, err := s.Process(pids[10], false)
	if err != nil {
		t.Error(err)
	}
	if len(p.EntityId) != 32 {
		t.Errorf("entityID is not correct")
	}

	_, err = s.Process(rng.RandomBytes(32), false)
	if err == nil {
		t.Errorf("process must not exist")
	}

	totalVotes, err := s.VoteCount(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))
	totalVotes, err = s.VoteCount(true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))

	votes := s.CountVotes(pids[40], false)
	if votes != 10 {
		t.Errorf("missing votes for process %x (got %d expected %d)", pids[40], votes, 10)
	}
	nullifiers := s.EnvelopeList(pids[50], 0, 20, false)
	if len(nullifiers) != 10 {
		t.Errorf("missing vote nullifiers (got %d expected %d)", len(nullifiers), 10)
	}
	nullifiers = s.EnvelopeList(pids[50], 0, 5, false)
	if len(nullifiers) != 5 {
		t.Errorf("missing vote nullifiers (got %d expected %d)", len(nullifiers), 5)
	}
}

type Listener struct {
	processStart [][][]byte
}

func (l *Listener) OnVote(vote *models.Vote, txIndex int32)                                      {}
func (l *Listener) OnNewTx(blockHeight uint32, txIndex int32)                                    {}
func (l *Listener) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32)       {}
func (l *Listener) OnProcessStatusChange(pid []byte, status models.ProcessStatus, txIndex int32) {}
func (l *Listener) OnCancel(pid []byte, txIndex int32)                                           {}
func (l *Listener) OnProcessKeys(pid []byte, encryptionPub string, txIndex int32)                {}
func (l *Listener) OnRevealKeys(pid []byte, encryptionPriv string, txIndex int32)                {}
func (l *Listener) OnProcessResults(pid []byte, results *models.ProcessResult, txIndex int32) error {
	return nil
}
func (l *Listener) OnProcessesStart(pids [][]byte) {
	l.processStart = append(l.processStart, pids)
}
func (l *Listener) Commit(height uint32) (err error) {
	return nil
}
func (l *Listener) Rollback() {}

func TestOnProcessStart(t *testing.T) {
	rng := newRandom(0)
	log.Init("info", "stdout")
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	listener := &Listener{}
	s.AddEventListener(listener)

	doBlock := func(height uint32, fn func()) {
		s.Rollback()
		s.SetHeight(height)
		fn()
		_, err := s.Save()
		qt.Assert(t, err, qt.IsNil)
	}

	pid := rng.RandomBytes(32)
	startBlock := uint32(4)
	doBlock(1, func() {
		censusURI := "ipfs://foobar"
		p := &models.Process{
			EntityId:   rng.RandomBytes(32),
			CensusURI:  &censusURI,
			ProcessId:  pid,
			StartBlock: startBlock,
			Mode: &models.ProcessMode{
				PreRegister: true,
			},
			EnvelopeType: &models.EnvelopeType{
				Anonymous: true,
			},
		}
		qt.Assert(t, s.AddProcess(p), qt.IsNil)
	})

	for i := uint32(2); i < 6; i++ {
		doBlock(i, func() {
			if i < startBlock {
				// Create a key with the last byte at 0 to make
				// sure it fits in the Poseidon field
				key := [32]byte{}
				copy(key[:31], rng.RandomBytes(31))
				err := s.AddToRollingCensus(pid, key[:], nil)
				qt.Assert(t, err, qt.IsNil)
			}
		})
		if i >= startBlock {
			qt.Assert(t, listener.processStart, qt.DeepEquals, [][][]byte{{pid}})
		}
	}
}
