// Copyright 2025 Upbound Inc.
// All rights reserved

package model

import (
	"sync"
	"time"
)

const errorShowDuration = 3 * time.Second

type TopLevel struct {
	lock  sync.RWMutex
	err   error
	errTS time.Time
}

func (t *TopLevel) SetError(err error) {
	if err == nil {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	t.errTS = time.Now()
}

func (t *TopLevel) Error() error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if time.Since(t.errTS) > errorShowDuration {
		return nil
	}
	return t.err
}
