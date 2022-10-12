package routerlock

import (
	"sync"

	"github.com/gin-gonic/gin"
)

type config struct {
	sync.RWMutex

	identifier string
	skipPaths  map[string]struct{}
}

type configMap struct {
	sync.Mutex

	m map[string]*config
}

var globalConfigMap = &configMap{m: make(map[string]*config)}

// RouterLock can lock the access of a group paths.
//
// The general path form represented as below:
//
// [scheme:][//[userinfo@]host][/]path[?query][#fragment]
//
// Param 'id' is a global identifier of the lock.
// If 'id' is already defined, new skipped path will be merged into the old.
//
// Param 'skipPaths' define wihch paths should not be locked for this lock.
func RouterLock(id string, skipPaths ...string) gin.HandlerFunc {
	globalConfigMap.Lock()
	defer globalConfigMap.Unlock()

	conf, ok := globalConfigMap.m[id]
	if !ok {
		conf = &config{
			identifier: id,
			skipPaths:  make(map[string]struct{}, len(skipPaths)),
		}
		globalConfigMap.m[id] = conf
	}
	for _, path := range skipPaths {
		conf.skipPaths[path] = struct{}{}
	}

	return func(c *gin.Context) {
		skip := conf.skipPaths
		if _, ok := skip[c.Request.URL.Path]; !ok {
			return
		}
		conf.Lock()
		defer conf.Unlock()
		c.Next()
	}
}
