package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	ErrNotFound       = errors.New("repository: not found")
	ErrConflict       = errors.New("repository: conflict")
	ErrOptimisticLock = errors.New("repository: optimistic lock")
	ErrInvalid        = errors.New("repository: invalid")
	ErrStoreClosed    = errors.New("repository: store closed")
)

type Item struct {
	ID        string
	Kind      string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Data      map[string]any
}
type Op struct {
	Kind          string
	ID            string
	Item          *Item
	ExpectVersion int64
	Delete        bool
}
type OpResult struct {
	Index   int
	ID      string
	Success bool
	Error   string
}
type Store interface {
	Load() (map[string]any, error)
	Save(map[string]any) error
	Get(ctx context.Context, kind, id string) (Item, error)
	Put(ctx context.Context, item Item) (Item, error)
	Delete(ctx context.Context, kind, id string) error
	List(ctx context.Context, kind string) ([]Item, error)
	Batch(ctx context.Context, ops []Op) ([]OpResult, error)
	Begin(ctx context.Context) (Transaction, error)
	Close() error
}
type Transaction interface {
	Get(kind, id string) (Item, error)
	Put(item Item) error
	Delete(kind, id string) error
	Commit(ctx context.Context) error
	Rollback() error
}

func checkCtx(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalid
	}
	return ctx.Err()
}
func validateItem(item *Item) error {
	if item == nil {
		return ErrInvalid
	}
	if item.ID == "" || item.Kind == "" {
		return ErrInvalid
	}
	if item.Version < 0 {
		return ErrInvalid
	}
	return nil
}
func applyVersion(existing Item, exists bool, item *Item) error {
	if !exists {
		if item.Version <= 0 {
			item.Version = 1
		}
		return nil
	}
	if item.Version != existing.Version+1 {
		return ErrOptimisticLock
	}
	return nil
}
func keyOf(kind, id string) string { return kind + "/" + id }
func parseKey(key string) (string, string, bool) {
	idx := 0
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}
func cloneItem(item Item) Item {
	b, err := json.Marshal(item)
	if err != nil {
		return Item{}
	}
	out := Item{}
	if err := json.Unmarshal(b, &out); err != nil {
		return Item{}
	}
	return out
}
func cloneItems(items map[string]map[string]Item) map[string]map[string]Item {
	out := map[string]map[string]Item{}
	if items == nil {
		return out
	}
	b, err := json.Marshal(items)
	if err != nil {
		return out
	}
	var decoded map[string]map[string]Item
	if err := json.Unmarshal(b, &decoded); err != nil {
		return out
	}
	if decoded != nil {
		return decoded
	}
	return out
}
func itemsFromData(data map[string]any) (map[string]map[string]Item, bool, error) {
	out := map[string]map[string]Item{}
	if data == nil {
		return out, false, nil
	}
	v, ok := data["items"]
	if !ok {
		return out, false, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false, err
	}
	var decoded map[string]map[string]Item
	if err := json.Unmarshal(b, &decoded); err != nil {
		return nil, false, err
	}
	if decoded == nil {
		decoded = out
	}
	return decoded, true, nil
}
func itemsToData(items map[string]map[string]Item) map[string]any {
	return map[string]any{"items": cloneItems(items)}
}

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]map[string]Item
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{items: map[string]map[string]Item{}} }
func (m *MemoryStore) Load() (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return itemsToData(m.items), nil
}
func (m *MemoryStore) Save(data map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	items, ok, err := itemsFromData(data)
	if err != nil {
		return err
	}
	if ok {
		m.items = items
	}
	return nil
}
func (m *MemoryStore) Get(ctx context.Context, kind, id string) (Item, error) {
	if err := checkCtx(ctx); err != nil {
		return Item{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	kindItems, ok := m.items[kind]
	if !ok {
		return Item{}, ErrNotFound
	}
	item, ok := kindItems[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return cloneItem(item), nil
}
func (m *MemoryStore) Put(ctx context.Context, item Item) (Item, error) {
	if err := checkCtx(ctx); err != nil {
		return Item{}, err
	}
	if err := validateItem(&item); err != nil {
		return Item{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.items == nil {
		m.items = map[string]map[string]Item{}
	}
	kindItems, ok := m.items[item.Kind]
	if !ok {
		kindItems = map[string]Item{}
		m.items[item.Kind] = kindItems
	}
	existing, exists := kindItems[item.ID]
	if err := applyVersion(existing, exists, &item); err != nil {
		return Item{}, err
	}
	if !exists {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	saved := cloneItem(item)
	kindItems[item.ID] = saved
	return cloneItem(saved), nil
}
func (m *MemoryStore) Delete(ctx context.Context, kind, id string) error {
	if err := checkCtx(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	kindItems, ok := m.items[kind]
	if !ok {
		return ErrNotFound
	}
	if _, ok := kindItems[id]; !ok {
		return ErrNotFound
	}
	delete(kindItems, id)
	if len(kindItems) == 0 {
		delete(m.items, kind)
	}
	return nil
}
func (m *MemoryStore) List(ctx context.Context, kind string) ([]Item, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Item{}
	kindItems := m.items[kind]
	for _, item := range kindItems {
		out = append(out, cloneItem(item))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	return out, nil
}
func (m *MemoryStore) Batch(ctx context.Context, ops []Op) ([]OpResult, error) {
	results := make([]OpResult, len(ops))
	for i, op := range ops {
		if err := checkCtx(ctx); err != nil {
			results[i] = OpResult{Index: i, ID: op.ID, Error: err.Error()}
			continue
		}
		switch {
		case op.Delete:
			if err := m.Delete(ctx, op.Kind, op.ID); err != nil {
				results[i] = OpResult{Index: i, ID: op.ID, Error: err.Error()}
			} else {
				results[i] = OpResult{Index: i, ID: op.ID, Success: true}
			}
		case op.Item != nil:
			if _, err := m.Put(ctx, *op.Item); err != nil {
				results[i] = OpResult{Index: i, ID: op.ID, Error: err.Error()}
			} else {
				results[i] = OpResult{Index: i, ID: op.ID, Success: true}
			}
		default:
			results[i] = OpResult{Index: i, ID: op.ID, Error: ErrInvalid.Error()}
		}
	}
	return results, nil
}
func (m *MemoryStore) Begin(ctx context.Context) (Transaction, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	snapshot := cloneItems(m.items)
	work := cloneItems(m.items)
	m.mu.RUnlock()
	return &memoryTransaction{mem: m, snapshot: snapshot, work: work, changed: map[string]string{}, deleted: map[string]string{}, ctx: ctx}, nil
}
func (m *MemoryStore) Close() error { return nil }

type memoryTransaction struct {
	mem      *MemoryStore
	snapshot map[string]map[string]Item
	work     map[string]map[string]Item
	changed  map[string]string
	deleted  map[string]string
	ctx      context.Context
}

func (t *memoryTransaction) Get(kind, id string) (Item, error) {
	kindItems, ok := t.work[kind]
	if !ok {
		return Item{}, ErrNotFound
	}
	item, ok := kindItems[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return cloneItem(item), nil
}
func (t *memoryTransaction) Put(item Item) error {
	if err := validateItem(&item); err != nil {
		return err
	}
	now := time.Now()
	if t.work[item.Kind] == nil {
		t.work[item.Kind] = map[string]Item{}
	}
	existing, exists := t.work[item.Kind][item.ID]
	if !exists {
		if item.Version <= 0 {
			item.Version = 1
		}
		item.CreatedAt = now
	} else {
		if err := applyVersion(existing, true, &item); err != nil {
			return err
		}
		item.CreatedAt = existing.CreatedAt
	}
	item.UpdatedAt = now
	t.work[item.Kind][item.ID] = cloneItem(item)
	t.changed[keyOf(item.Kind, item.ID)] = keyOf(item.Kind, item.ID)
	delete(t.deleted, keyOf(item.Kind, item.ID))
	return nil
}
func (t *memoryTransaction) Delete(kind, id string) error {
	kindItems, ok := t.work[kind]
	if !ok {
		return ErrNotFound
	}
	if _, ok := kindItems[id]; !ok {
		return ErrNotFound
	}
	delete(kindItems, id)
	if len(kindItems) == 0 {
		delete(t.work, kind)
	}
	key := keyOf(kind, id)
	t.deleted[key] = key
	delete(t.changed, key)
	return nil
}
func (t *memoryTransaction) Commit(ctx context.Context) error {
	if err := checkCtx(ctx); err != nil {
		return err
	}
	t.mem.mu.Lock()
	defer t.mem.mu.Unlock()
	if t.mem.items == nil {
		t.mem.items = map[string]map[string]Item{}
	}
	for key := range t.changed {
		kind, id, ok := parseKey(key)
		if !ok {
			return ErrInvalid
		}
		curExists := false
		if curItems, ok := t.mem.items[kind]; ok {
			if _, ok := curItems[id]; ok {
				curExists = true
			}
		}
		snapExists := false
		if snapItems, ok := t.snapshot[kind]; ok {
			if _, ok := snapItems[id]; ok {
				snapExists = true
			}
		}
		if curExists && snapExists {
			cur := t.mem.items[kind][id]
			snap := t.snapshot[kind][id]
			if cur.Version != snap.Version {
				return ErrOptimisticLock
			}
		}
		if !curExists && snapExists {
			return ErrOptimisticLock
		}
		workItem, ok := t.work[kind][id]
		if !ok {
			return ErrInvalid
		}
		if t.mem.items[kind] == nil {
			t.mem.items[kind] = map[string]Item{}
		}
		t.mem.items[kind][id] = cloneItem(workItem)
	}
	for key := range t.deleted {
		kind, id, ok := parseKey(key)
		if !ok {
			return ErrInvalid
		}
		curExists := false
		if curItems, ok := t.mem.items[kind]; ok {
			if _, ok := curItems[id]; ok {
				curExists = true
			}
		}
		snapExists := false
		if snapItems, ok := t.snapshot[kind]; ok {
			if _, ok := snapItems[id]; ok {
				snapExists = true
			}
		}
		if !snapExists && !curExists {
			return ErrInvalid
		}
		if curExists && snapExists {
			cur := t.mem.items[kind][id]
			snap := t.snapshot[kind][id]
			if cur.Version != snap.Version {
				return ErrOptimisticLock
			}
		}
		if t.mem.items[kind] != nil {
			delete(t.mem.items[kind], id)
		}
		if t.mem.items[kind] != nil && len(t.mem.items[kind]) == 0 {
			delete(t.mem.items, kind)
		}
	}
	t.changed = map[string]string{}
	t.deleted = map[string]string{}
	return nil
}
func (t *memoryTransaction) Rollback() error {
	t.work = cloneItems(t.snapshot)
	t.changed = map[string]string{}
	t.deleted = map[string]string{}
	return nil
}

type FileStore struct {
	path   string
	file   string
	mem    *MemoryStore
	mu     sync.Mutex
	closed bool
}

func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		path = "repository"
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	fs := &FileStore{path: path, file: filepath.Join(path, "repository.json"), mem: NewMemoryStore()}
	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}
func (f *FileStore) load() error {
	data, err := os.ReadFile(f.file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var items map[string]map[string]Item
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	if items == nil {
		items = map[string]map[string]Item{}
	}
	f.mem.mu.Lock()
	f.mem.items = items
	f.mem.mu.Unlock()
	return nil
}
func (f *FileStore) persistLocked() error {
	if f.closed {
		return nil
	}
	f.mem.mu.RLock()
	data, err := json.Marshal(f.mem.items)
	f.mem.mu.RUnlock()
	if err != nil {
		return err
	}
	tmp := f.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, f.file); err != nil {
		return err
	}
	return nil
}
func (f *FileStore) persist() error { f.mu.Lock(); defer f.mu.Unlock(); return f.persistLocked() }
func (f *FileStore) Save(data map[string]any) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return ErrStoreClosed
	}
	if err := f.mem.Save(data); err != nil {
		f.mu.Unlock()
		return err
	}
	err := f.persistLocked()
	f.mu.Unlock()
	return err
}
func (f *FileStore) Load() (map[string]any, error) { return f.mem.Load() }
func (f *FileStore) Get(ctx context.Context, kind, id string) (Item, error) {
	return f.mem.Get(ctx, kind, id)
}
func (f *FileStore) Put(ctx context.Context, item Item) (Item, error) {
	saved, err := f.mem.Put(ctx, item)
	if err != nil {
		return Item{}, err
	}
	if err := f.persist(); err != nil {
		return saved, err
	}
	return saved, nil
}
func (f *FileStore) Delete(ctx context.Context, kind, id string) error {
	if err := f.mem.Delete(ctx, kind, id); err != nil {
		return err
	}
	return f.persist()
}
func (f *FileStore) List(ctx context.Context, kind string) ([]Item, error) {
	return f.mem.List(ctx, kind)
}
func (f *FileStore) Batch(ctx context.Context, ops []Op) ([]OpResult, error) {
	results, err := f.mem.Batch(ctx, ops)
	if err != nil {
		return nil, err
	}
	success := false
	for _, r := range results {
		if r.Success {
			success = true
		}
	}
	if success {
		if err := f.persist(); err != nil {
			return results, err
		}
	}
	return results, nil
}
func (f *FileStore) Begin(ctx context.Context) (Transaction, error) {
	tx, err := f.mem.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &fileTransaction{tx: tx.(*memoryTransaction), store: f}, nil
}
func (f *FileStore) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.persistLocked()
}

type fileTransaction struct {
	tx    *memoryTransaction
	store *FileStore
}

func (ft *fileTransaction) Get(kind, id string) (Item, error) { return ft.tx.Get(kind, id) }
func (ft *fileTransaction) Put(item Item) error               { return ft.tx.Put(item) }
func (ft *fileTransaction) Delete(kind, id string) error      { return ft.tx.Delete(kind, id) }
func (ft *fileTransaction) Commit(ctx context.Context) error {
	if err := ft.tx.Commit(ctx); err != nil {
		return err
	}
	return ft.store.persist()
}
func (ft *fileTransaction) Rollback() error { return ft.tx.Rollback() }
