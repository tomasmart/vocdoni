package urlapi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
)

// Namespace is the string identifier for the httprouter used by the URLAPI
const Namespace = "urlAPI"

type URLAPI struct {
	PrivateCalls uint64
	PublicCalls  uint64
	BaseRoute    string

	router       *httprouter.HTTProuter
	api          *bearerstdapi.BearerStandardAPI
	scrutinizer  *scrutinizer.Scrutinizer
	vocapp       *vochain.BaseApplication
	metricsagent *metrics.Agent
	vocinfo      *vochaininfo.VochainInfo
	basePath     int
}

func NewURLAPI(router *httprouter.HTTProuter, baseRoute string) (*URLAPI, error) {
	if router == nil {
		return nil, fmt.Errorf("httprouter is nil")
	}
	if len(baseRoute) == 0 || baseRoute[0] != '/' {
		return nil, fmt.Errorf("invalid base route (%s), it must start with /", baseRoute)
	}
	// Remove trailing slash
	if len(baseRoute) > 1 {
		baseRoute = strings.TrimSuffix(baseRoute, "/")
	}
	urlapi := URLAPI{
		BaseRoute: baseRoute,
		router:    router,
		basePath:  strings.Count(baseRoute, "/"),
	}
	var err error
	urlapi.api, err = bearerstdapi.NewBearerStandardAPI(router, baseRoute)
	if err != nil {
		return nil, err
	}

	if err := urlapi.api.RegisterMethod(
		"/entities/#entity/processes/#status",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		urlapi.entitiesHandler,
	); err != nil {
		return nil, err
	}

	return &urlapi, nil
}

func (u *URLAPI) sendError(ctx *httprouter.HTTPContext, errMsgs ...string) {
	msg, err := json.Marshal(&ErrorMsg{Error: strings.Join(errMsgs, ", ")})
	if err != nil {
		log.Warnf("cannot send error message %v", err)
		return
	}
	if err = ctx.Send(msg); err != nil {
		log.Warnf("cannot send error message %v", err)
	}
}

// https://server/v1/pub/entities/<entityId>/processes/active
func (u *URLAPI) entitiesHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	entityID, err := hex.DecodeString(util.TrimHex(msg.Vars["entity"]))
	if err != nil {
		return fmt.Errorf("entityID cannot be decoded")
	}

	switch msg.Vars["status"] {
	case "active":
		pids, err := u.scrutinizer.ProcessList(entityID, 0, 128, "", 0, "", "READY", false)
		if err != nil {
			return fmt.Errorf("cannot fetch process list (%w)", err)
		}
		processes, err := u.getProcessSummaryList(pids...)
		if err != nil {
			return err
		}
		data, err := json.Marshal(&EntitiesMsg{
			EntityID:  types.HexBytes(entityID),
			Processes: processes,
		})
		if err != nil {
			return fmt.Errorf("error marshaling JSON (%w)", err)
		}
		if err = ctx.Send(data); err != nil {
			log.Warn(err)
		}
	default:
		return fmt.Errorf("missing status parameter or unknown")
	}
	return nil
}

/*
func (u *URLAPI) entitiesHandler(msg httprouter.Message) {
	if len(msg.Path) < u.basePath+2 {
		u.sendError(msg.Context, "missing url arguments")
		return
	}
	entityID, err := hex.DecodeString(util.TrimHex(msg.Path[u.basePath+1]))
	if err != nil {
		u.sendError(msg.Context, "entityID cannot be decoded")
		return
	}
	switch msg.Path[u.basePath+2] {
	case "processes":
		if len(msg.Path) < u.basePath+3 {
			u.sendError(msg.Context, "missing url arguments")
			return
		}
		switch msg.Path[u.basePath+3] {
		case "active":
			pids, err := u.scrutinizer.ProcessList(entityID, 0, 128, "", 0, "", "READY", false)
			if err != nil {
				u.sendError(msg.Context, "cannot fetch process list", err.Error())
				return
			}
			processes, err := u.getProcessSummaryList(pids...)
			if err != nil {
				u.sendError(msg.Context, err.Error())
				return
			}
			data, err := json.Marshal(&EntitiesMsg{
				EntityID:  types.HexBytes(entityID),
				Processes: processes,
			})
			if err != nil {
				u.sendError(msg.Context, "error marshaling JSON", err.Error())
				return
			}
			if err = msg.Context.Send(data); err != nil {
				log.Warn(err)
			}
		}

	}
}
*/

func (u *URLAPI) getProcessSummaryList(pids ...[]byte) ([]*ProcessSummary, error) {
	processes := []*ProcessSummary{}
	for _, p := range pids {
		procInfo, err := u.scrutinizer.ProcessInfo(p)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch process info (%v)", err.Error())
		}
		processes = append(processes, &ProcessSummary{
			ProcessID: procInfo.ID,
			Status:    models.ProcessStatus_name[procInfo.Status],
			StartDate: procInfo.CreationTime,
		})
	}
	return processes, nil
}

type ErrorMsg struct {
	Error string `json:"error"`
}

type EntitiesMsg struct {
	EntityID  types.HexBytes    `json:"entityID"`
	Processes []*ProcessSummary `json:"processes,omitempty"`
}

type ProcessSummary struct {
	ProcessID types.HexBytes `json:"processId,omitempty"`
	Status    string         `json:"status,omitempty"`
	StartDate time.Time      `json:"startDate,omitempty"`
	EndDate   time.Time      `json:"endDate,omitempty"`
}
