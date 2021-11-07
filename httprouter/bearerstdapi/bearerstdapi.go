package bearerstdapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
)

const (
	MethodAccessTypePrivate = "private"
	MethodAccessTypePublic  = "public"
	MethodAccessTypeAdmin   = "admin"

	namespace    = "bearerStdAPI"
	bearerPrefix = "Bearer "
)

// BearerStandardAPI is a namespace handler for the httpRouter with Bearer authorization
type BearerStandardAPI struct {
	router         *httprouter.HTTProuter
	basePath       string
	authTokens     sync.Map
	adminToken     string
	adminTokenLock sync.RWMutex
}

// BearerStandardAPIdata is the data type used by the BearerStandardAPI.
// On handler functions Message.Data can be cast safely to this type.
type BearerStandardAPIdata struct {
	Data      []byte
	AuthToken string
	Vars      map[string]string
}

type BearerStdAPIhandler = func(*BearerStandardAPIdata, *httprouter.HTTPContext) error

type ErrorMsg struct {
	Error string `json:"error"`
}

// NewBearerStandardAPI returns a BearerStandardAPI initialized type
func NewBearerStandardAPI(router *httprouter.HTTProuter, baseRoute string) (*BearerStandardAPI, error) {
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
	bsa := BearerStandardAPI{router: router, basePath: baseRoute}
	router.AddNamespace(namespace, &bsa)
	return &bsa, nil

}

// AuthorizeRequest is a function for the RouterNamespace interface.
// On private handlers checks if the supplied bearer token have still request credits
func (b *BearerStandardAPI) AuthorizeRequest(data interface{}, isAdmin bool) (bool, error) {
	msg, ok := data.(*BearerStandardAPIdata)
	if !ok {
		panic("type is not bearerStandardApi")
	}
	if isAdmin {
		b.adminTokenLock.RLock()
		defer b.adminTokenLock.RUnlock()
		if msg.AuthToken != b.adminToken {
			return false, fmt.Errorf("admin token not valid")
		}
		return true, nil
	}
	remainingReqs, ok := b.authTokens.Load(msg.AuthToken)
	if !ok || remainingReqs.(int64) < 1 {
		return false, fmt.Errorf("no more requests available")
	}
	b.authTokens.Store(msg.AuthToken, remainingReqs.(int64)-1)
	return true, nil
}

// ProcessData is a function for the RouterNamespace interface.
// The body of the http requests and the bearer auth token are readed.
func (b *BearerStandardAPI) ProcessData(req *http.Request) (interface{}, error) {
	//req.URL.Path
	respBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP connection closed: (%v)", err)
	}
	return &BearerStandardAPIdata{
		Data:      respBody,
		AuthToken: strings.TrimPrefix(req.Header.Get("Authorization"), bearerPrefix),
	}, nil
}

func (b *BearerStandardAPI) RegisterMethod(pattern, HTTPmethod string, accessType string, handler BearerStdAPIhandler) error {
	//pattern = strings.TrimPrefix(pattern, "/")
	elementsCount := 0
	elementsMap := make(map[int]string)
	routerPattern := []string{}
	for i, element := range strings.Split(pattern, "/") {
		if strings.HasPrefix(element, "#") {
			elementsMap[i] = strings.TrimPrefix(element, "#")
			elementsCount++
			routerPattern = append(routerPattern, fmt.Sprintf("{%s}", elementsMap[i]))
		} else {
			routerPattern = append(routerPattern, element)
		}
	}

	routerHandler := func(msg httprouter.Message) {
		bsaMsg := msg.Data.(*BearerStandardAPIdata)
		bsaMsg.Vars = make(map[string]string, elementsCount)
		for i, e := range msg.Path {
			varName, ok := elementsMap[i]
			if ok {
				bsaMsg.Vars[varName] = e
			}
		}

		if err := handler(bsaMsg, msg.Context); err != nil {
			data, err := json.Marshal(&ErrorMsg{Error: err.Error()})
			if err != nil {
				log.Warn(err)
				return
			}
			if err := msg.Context.Send(data); err != nil {
				log.Warn(err)
			}
		}
	}

	path := b.basePath + strings.Join(routerPattern, "/")
	switch accessType {
	case "public":
		b.router.AddPublicHandler(namespace, path, HTTPmethod, routerHandler)
	case "private":
		b.router.AddPrivateHandler(namespace, path, HTTPmethod, routerHandler)
	case "admin":
		b.router.AddAdminHandler(namespace, path, HTTPmethod, routerHandler)
	default:
		return fmt.Errorf("method access type not implemented: %s", accessType)
	}
	log.Infof("registered %s %s method for path %s", HTTPmethod, accessType, path)
	return nil
}

// SetAdminToken sets the bearer admin token capable to execute admin handlers
func (b *BearerStandardAPI) SetAdminToken(bearerToken string) {
	b.adminTokenLock.Lock()
	defer b.adminTokenLock.Unlock()
	b.adminToken = bearerToken
}

// AddAuthToken adds a new bearer token capable to perform up to n requests
func (b *BearerStandardAPI) AddAuthToken(bearerToken string, requests int64) {
	b.authTokens.Store(bearerToken, requests)
}

// DelAuthToken removes a bearer token (will be not longer valid)
func (b *BearerStandardAPI) DelAuthToken(bearerToken string) {
	b.authTokens.Delete(bearerToken)
}

// GetAuthTokens returns the number of pending requests credits for a bearer token
func (b *BearerStandardAPI) GetAuthTokens(bearerToken string) int64 {
	ts, ok := b.authTokens.Load(bearerToken)
	if !ok {
		return 0
	}
	return ts.(int64)
}
