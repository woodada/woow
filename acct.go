package woow

import (
    "sync"
)

var _ sync.Locker = (*AccountStore)(nil) // nocopy
func (this AccountStore) Lock()         {} // nocopy
func (this AccountStore) Unlock()       {} // nocopy

// 线程安全的账号/密码
type AccountStore struct {
    d map[string]string
    m sync.RWMutex
}

func NewAccountStore(accts map[string]string) *AccountStore {
    d := make(map[string]string)
    for k, v := range accts {
        d[k] = v
    }
    return &AccountStore{d: d}
}

func (this *AccountStore) Upsert(username, passwd string) {
    this.m.Lock()
    defer this.m.Unlock()
    this.d[username] = passwd
}

// 0-成功 1-用户不存在 2-密码错误
func (this *AccountStore) Compare(username, passwd string) int {
    this.m.RLock()
    defer this.m.RUnlock()
    p, ok := this.d[username]
    if ok && p == passwd {
        return 0
    }

    if !ok {
        return 1
    }

    return 2
}

func (this *AccountStore) Size() int {
    this.m.RLock()
    defer this.m.RUnlock()
    return len(this.d)
}
