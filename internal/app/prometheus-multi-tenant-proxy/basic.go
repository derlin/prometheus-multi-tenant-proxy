package proxy

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/k8spin/prometheus-multi-tenant-proxy/internal/pkg"
)

const (
	//Namespaces Key used to pass prometheus tenant id though the middleware context
	Namespaces key = iota
	realm          = "Prometheus multi-tenant proxy"
)

// BasicAuth can be used as a middleware chain to authenticate users
// with Basic authentication before proxying a request
type BasicAuth struct {
	configLocation string
	config         *pkg.Authn
	configLock     *sync.RWMutex
}

// NewBasicAuth creates a BasicAuth, loading the Authn from configLocation
func NewBasicAuth(configLocation string) *BasicAuth {
	auth := &BasicAuth{
		configLocation: configLocation,
		configLock:     new(sync.RWMutex),
	}
	if !auth.Load() {
		os.Exit(1)
	}
	return auth
}

func newBasicAuthFromConfig(authn *pkg.Authn) *BasicAuth {
	// Load cannot be called!
	return &BasicAuth{
		config:     authn,
		configLock: new(sync.RWMutex),
	}
}

// Load loads or reload the Authn from the configuration file
func (auth *BasicAuth) Load() bool {
	temp, err := pkg.ParseConfig(&auth.configLocation)
	if err != nil {
		log.Printf("Could not parse config file %s: %v", auth.configLocation, err)
		return false
	}
	auth.configLock.Lock()
	auth.config = temp
	auth.configLock.Unlock()
	log.Print("Reloaded authn configuration from file")
	return true
}

// IsAuthorized uses the basic authentication and the Authn file to authenticate a user
// and return the namespace he has access to
func (auth *BasicAuth) IsAuthorized(r *http.Request) (bool, []string) {
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false, nil
	}
	return auth.isAuthorized(user, pass)
}

func (auth *BasicAuth) isAuthorized(user, pass string) (bool, []string) {
	authConfig := auth.getConfig()
	for _, v := range authConfig.Users {
		if subtle.ConstantTimeCompare([]byte(user), []byte(v.Username)) == 1 && subtle.ConstantTimeCompare([]byte(pass), []byte(v.Password)) == 1 {
			return true, append(v.Namespaces, v.Namespace)
		}
	}
	return false, nil
}

// WriteUnauthorisedResponse writes a 401 Unauthorized HTTP response with
// a redirect to basic authentication
func (auth *BasicAuth) WriteUnauthorisedResponse(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	w.WriteHeader(401)
	w.Write([]byte("Unauthorised\n"))
}

func (auth *BasicAuth) getConfig() *pkg.Authn {
	auth.configLock.RLock()
	defer auth.configLock.RUnlock()
	return auth.config
}
