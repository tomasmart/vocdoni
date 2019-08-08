package data

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"

	gocid "github.com/ipfs/go-cid"

	"net/http"
	"os"

	//"os/exec"
	"strings"

	ipfscmds "github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	ipfslog "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	ipfscluster "github.com/ipfs/ipfs-cluster"
	clusterapi "github.com/ipfs/ipfs-cluster/api"
	logging "github.com/whyrusleeping/go-logging"
	"gitlab.com/vocdoni/go-dvote/ipfs"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core/coreunix"
	crypto "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type IPFSHandle struct {
	nd      *ipfscore.IpfsNode
	cluster *ipfscluster.Cluster
	coreAPI coreiface.CoreAPI
	dataDir string
}

// check if ipfs base dir exists
func checkIPFSinit(bin string) (bool, error) {
	_, err := os.Stat(bin)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err

	// initCmd := exec.Command(bin, "config", "show")
	// return initCmd.Run()
}

//Init sets up an IPFS native node and cluster
func (i *IPFSHandle) Init(d *types.DataStore) error {
	lvl, err := logging.LogLevel(strings.ToUpper(d.ClusterCfg.ClusterLogLevel))
	if err != nil {
		log.Warn(err.Error())
	}
	ipfslog.SetAllLoggers(lvl)
	const programName = `dvote-ipfs`

	dirExists, err := checkIPFSinit(d.Datadir)
	if err != nil {
		log.Warn(err.Error())
		return errors.New("cannot check if IPFS dir exists")
	}
	if !dirExists {
		err = os.MkdirAll(d.Datadir, os.ModePerm)
	}

	go func() {
		log.Info(http.ListenAndServe("localhost:6060", nil))
	}()

	ipfs.InstallDatabasePlugins()
	ipfs.ConfigRoot = d.Datadir
	//check if needs init
	if !fsrepo.IsInitialized(ipfs.ConfigRoot) {
		err := ipfs.Init()
		if err != nil {
			log.Warn(err.Error())
			return err
		} else {
			log.Info("IPFS init done!")
		}
	}

	nd, coreAPI, err := ipfs.StartNode()
	if err != nil {
		log.Errorf("Error in StartNode: ", err)
	}
	log.Infof("Peer ID: %s", nd.Identity.Pretty())

	//start http
	cctx := ipfs.CmdCtx(nd, d.Datadir)
	cctx.ReqLog = &ipfscmds.ReqLog{}

	gatewayOpt := corehttp.GatewayOption(true, corehttp.WebUIPaths...)
	var opts = []corehttp.ServeOption{
		corehttp.CommandsOption(cctx),
		corehttp.WebUIOption,
		gatewayOpt,
	}

	go corehttp.ListenAndServe(nd, "/ip4/0.0.0.0/tcp/5001", opts...)

	i.nd = nd
	i.coreAPI = coreAPI
	i.dataDir = d.Datadir

	ipfs.ProgramName = programName
	log.Infof("ipfs init done!")

	if len(d.ClusterCfg.Secret) > 0 {
		log.Info("initializing ipfs cluster")
		clusterPath := d.Datadir + "/.cluster"
		d.ClusterCfg.PeerID = i.nd.Identity
		d.ClusterCfg.Private = i.nd.PrivateKey
		err = ipfs.InitCluster(clusterPath, "conf.json", "id.json", d.ClusterCfg)
		if err != nil {
			log.Fatalf("Error initializing ipfs cluster: %v", err)
		}
		ch := make(chan *ipfscluster.Cluster)
		go ipfs.RunCluster(d.ClusterCfg, ch)
		log.Debug("Cluster has run!!!")
		cluster := <-ch
		i.cluster = cluster
		if err != nil {
			log.Fatalf("Error running ipfs cluster: %v", err)
		}
	}
	return nil
}

//PublishFile publishes a file specified by root to ipfs (node)
func publishFile(root []byte, nd *ipfscore.IpfsNode) (string, error) {
	rootHash, err := addAndPin(nd, string(root))
	if err != nil {
		return "", err
	}
	return rootHash, nil
}

//PublishBytes publishes a file containing msg to ipfs (node)
func publishBytes(msg []byte, fileDir string, nd *ipfscore.IpfsNode) (string, error) {
	filePath := fmt.Sprintf("%s/%x", fileDir, crypto.HashRaw(string(msg)))
	log.Infof("Publishing file: %s", filePath)
	err := ioutil.WriteFile(filePath, msg, 0666) //these should be more restrictive
	rootHash, err := addAndPin(nd, filePath)
	if err != nil {
		return "", err
	}
	return rootHash, nil

}

//This should disambiguate publish mode (cluster vs node, and call appropraite func)
func (i *IPFSHandle) Publish(msg []byte) (string, error) {
	roothash, err := publishBytes(msg, i.dataDir, i.nd)
	if err != nil {
		return "", err
	}
	if i.cluster != nil {
		i.Pin(roothash)
	}
	return roothash, nil
}

//This is cluster add (currently non-functional due to MIME-type requirement)
func addFile(filePath string, cluster *ipfscluster.Cluster) (rootHash string, err error) {
	reader, err := os.Open(filePath)
	if err != nil {
		log.Error("Could not open file: ", filePath)
	}
	part := strings.NewReader("<start of file>")
	_ = io.MultiReader(part, reader)
	multi := multipart.NewReader(reader, "")
	cid, err := cluster.AddFile(multi, &clusterapi.AddParams{
		Recursive:      true,
		Layout:         "balanced", // corresponds to balanced layout
		Chunker:        "size-262144",
		RawLeaves:      false,
		Hidden:         false,
		Wrap:           false,
		Shard:          true,
		Progress:       false,
		CidVersion:     0,
		HashFun:        "sha2-256",
		StreamChannels: true,
		NoCopy:         false,
		PinOptions: clusterapi.PinOptions{
			ReplicationFactorMin: 1,
			ReplicationFactorMax: 4,
			Name:                 "",
			ShardSize:            clusterapi.DefaultShardSize,
		},
	})
	return cid.String(), err
}

//this is node add and node pin
func addAndPin(n *ipfscore.IpfsNode, root string) (rootHash string, err error) {
	defer n.Blockstore.PinLock().Unlock()
	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, false, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fileAdder, err := coreunix.NewAdder(context.Background(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	node, err := fileAdder.AddAllAndPin(f)
	if err != nil {
		return "", err
	}
	return node.Cid().String(), nil
}

//This is cluster pin
func (i *IPFSHandle) Pin(path string) error {
	cid, err := gocid.Decode(path)
	if err != nil {
		return err
	}
	pin := clusterapi.PinCid(cid)
	pin.PinOptions.ReplicationFactorMax = -1
	return i.cluster.Pin(context.Background(), pin)
}

//This is cluster unpin
func (i *IPFSHandle) Unpin(path string) error {
	p := corepath.New(path)
	rp, err := i.coreAPI.ResolvePath(context.Background(), p)
	if err != nil {
		return err
	}
	return i.cluster.Unpin(context.Background(), rp.Cid())
}

//This is cluster listpins
func (i *IPFSHandle) ListPins() (map[string]string, error) {
	pins, err := i.cluster.Pins(context.Background())
	if err != nil {
		return nil, err
	}
	var pinMap map[string]string
	pinMap = make(map[string]string)
	for _, p := range pins {
		pinMap[p.Cid.String()] = p.Type.String()
	}
	return pinMap, nil
}

//This is node retrieve
func (i *IPFSHandle) Retrieve(path string) ([]byte, error) {
	ctx := context.Background()

	if !strings.HasPrefix(path, "/ipfs/") {
		path = "/ipfs/" + path
	}

	pth := corepath.New(path)

	nd, err := i.coreAPI.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}

	r, ok := nd.(files.File)
	if !ok {
		return nil, errors.New("Received incorrect type from Unixfs().Get()")
	}

	return ioutil.ReadAll(r)
}
