package urlapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
)

func TestCensus(t *testing.T) {
	log.Init("debug", "stdout")
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "census"))
	qt.Assert(t, err, qt.IsNil)

	t.Logf("address: %s", addr)

	api, err := NewURLAPI(&router, "/", t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	storage := data.IPFSHandle{}
	api.Attach(nil, nil, nil, data.Storage(&storage))
	qt.Assert(t, api.EnableHandlers(CensusHandler), qt.IsNil)

	token1 := uuid.New()
	c := newTestHTTPclient(t, addr, &token1)

	// create a new census
	resp, code := c.request("GET", nil, "create", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := fmt.Sprintf("%x", censusData.CensusID)

	// check weight and size (must be zero)
	resp, code = c.request("GET", nil, id1, "weight")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "0")

	resp, code = c.request("GET", nil, id1, "size")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Size, qt.Equals, uint64(0))

	// add some keys and weights
	// add a bunch of keys and values (weights)
	rnd := testutil.NewRandom(1)
	weight := 0
	index := uint64(0)
	for i := 1; i < 11; i++ {
		_, code = c.request("GET", nil, id1, "add", fmt.Sprintf("%x", rnd.RandomBytes(32)), fmt.Sprintf("%d", i))
		qt.Assert(t, code, qt.Equals, 200)
		weight += i
		index++
	}

	// check again weight and size
	resp, code = c.request("GET", nil, id1, "weight")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, fmt.Sprintf("%d", weight))

	resp, code = c.request("GET", nil, id1, "size")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Size, qt.Equals, index)

	// add one more key and check proof
	key := rnd.RandomBytes(32)
	keyWeight := "100"
	_, code = c.request("GET", nil, id1, "add", fmt.Sprintf("%x", key), keyWeight)
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.request("GET", nil, id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	id2 := fmt.Sprintf("%x", censusData.CensusID)

	resp, code = c.request("GET", nil, id2, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, keyWeight)

	proof := Census{
		Key:   key,
		Proof: censusData.Proof,
		Value: censusData.Value,
	}
	data, err := json.Marshal(proof)
	qt.Assert(t, err, qt.IsNil)
	resp, code = c.request("POST", data, id2, "verify")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Valid, qt.IsTrue)

	// try an invalid proof
	proof.Key = rnd.RandomBytes(32)
	data, err = json.Marshal(proof)
	qt.Assert(t, err, qt.IsNil)
	_, code = c.request("POST", data, id2, "verify")
	qt.Assert(t, code, qt.Equals, 400)

	// dump the tree
	resp, code = c.request("GET", nil, id1, "dump")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)

	// create a new one and try to import the previous dump
	resp2, code := c.request("GET", nil, "create", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp2, censusData), qt.IsNil)
	id3 := fmt.Sprintf("%x", censusData.CensusID)

	_, code = c.request("POST", resp, id3, "import")
	qt.Assert(t, code, qt.Equals, 200)

	// delete the first census
	_, code = c.request("GET", nil, id1, "delete")
	qt.Assert(t, code, qt.Equals, 200)

	// check the second census is still available
	_, code = c.request("GET", nil, id2, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 200)

	// check the first census is not available
	_, code = c.request("GET", nil, id1, "proof", fmt.Sprintf("%x", key))
	qt.Assert(t, code, qt.Equals, 400)
}
