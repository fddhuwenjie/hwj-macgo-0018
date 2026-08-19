package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNotFound          = errors.New("application: not found")
	ErrConflict          = errors.New("application: conflict")
	ErrIllegalTransition = errors.New("application: illegal transition")
	ErrOptimisticLock    = errors.New("application: optimistic lock")
	ErrExpired           = errors.New("application: expired")
	ErrInvalid           = errors.New("application: invalid")
	ErrStoreFailed       = errors.New("application: store failed")
)

const (
	StatusPreRegistered = "pre_registered"
	StatusOccupied      = "occupied"
	StatusCommitted     = "committed"
	StatusReleased      = "released"
	StatusFailed        = "failed"
	credentialActive    = "active"
	credentialCommitted = "committed"
	credentialReleased  = "released"
	credentialExpired   = "expired"
)

type Request struct {
	ID             string
	CallerID       string
	NamespaceID    string
	Key            string
	Digest         string
	Status         string
	Generation     int64
	Version        int64
	CredentialID   string
	ResultID       string
	LeaseExpiresAt time.Time
	CommittedAt    time.Time
	FailureReason  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
type Credential struct {
	ID         string
	RequestID  string
	Generation int64
	Status     string
	Version    int64
	ExpiresAt  time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
type Result struct {
	ID         string
	RequestID  string
	Generation int64
	Payload    map[string]any
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
type Replay struct {
	ID         string
	RequestID  string
	ResultID   string
	Generation int64
	HitAt      time.Time
	Version    int64
}

type Persister interface {
	Load() (map[string]any, error)
	Save(map[string]any) error
}
type loadedData struct {
	Requests    map[string]*Request
	Credentials map[string]*Credential
	Results     map[string]*Result
	Replays     map[string][]Replay
	Failures    map[string][]string
}
type UseCase struct {
	mu          sync.RWMutex
	now         func() time.Time
	idCounter   int64
	requests    map[string]*Request
	byKey       map[string]*Request
	credentials map[string]*Credential
	results     map[string]*Result
	replays     map[string][]Replay
	failures    map[string][]string
	store       any
}
type Service = UseCase

func NewUseCase(opts ...any) *UseCase     { return NewService(opts...) }
func NewApplication(opts ...any) *UseCase { return NewService(opts...) }
func NewService(opts ...any) *UseCase {
	s := &UseCase{now: time.Now, requests: map[string]*Request{}, byKey: map[string]*Request{}, credentials: map[string]*Credential{}, results: map[string]*Result{}, replays: map[string][]Replay{}, failures: map[string][]string{}}
	for _, opt := range opts {
		if p, ok := opt.(Persister); ok {
			s.store = p
			s.load(p)
		}
		if n, ok := opt.(func() time.Time); ok {
			s.now = n
		}
	}
	s.rebuildIndexLocked()
	return s
}

func (s *UseCase) rebuildIndexLocked() {
	s.byKey = map[string]*Request{}
	for _, r := range s.requests {
		if r != nil {
			s.byKey[s.keyScope(r.CallerID, r.NamespaceID, r.Key)] = r
		}
	}
}
func (s *UseCase) checkContext(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
func blank(v string) bool { return strings.TrimSpace(v) == "" }
func (s *UseCase) newID(prefix string) string {
	n := atomic.AddInt64(&s.idCounter, 1)
	return fmt.Sprintf("%s-%d-%d", prefix, s.nowFunc().UnixNano(), n)
}
func (s *UseCase) keyScope(callerID, namespaceID, key string) string {
	return callerID + "/" + namespaceID + "/" + key
}
func (s *UseCase) nowFunc() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}
func (s *UseCase) load(p Persister) {
	data, err := p.Load()
	if err != nil {
		return
	}
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	var ld loadedData
	if err := json.Unmarshal(b, &ld); err != nil {
		return
	}
	if ld.Requests != nil {
		s.requests = ld.Requests
	}
	if ld.Credentials != nil {
		s.credentials = ld.Credentials
	}
	if ld.Results != nil {
		s.results = ld.Results
	}
	if ld.Replays != nil {
		s.replays = ld.Replays
	}
	if ld.Failures != nil {
		s.failures = ld.Failures
	}
	s.rebuildIndexLocked()
}
func (s *UseCase) persist() error {
	if s.store == nil {
		return nil
	}
	p, ok := s.store.(Persister)
	if !ok {
		return nil
	}
	data := map[string]any{"requests": s.requests, "credentials": s.credentials, "results": s.results, "replays": s.replays, "failures": s.failures}
	return p.Save(data)
}
func clonePayload(m map[string]any) map[string]any {
	if len(m) == 0 {
		return map[string]any{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}
func cloneRequest(r *Request) Request {
	out := Request{}
	if r == nil {
		return out
	}
	b, err := json.Marshal(r)
	if err != nil {
		return out
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out
	}
	return out
}

type PreRegisterRequest struct {
	CallerID    string
	NamespaceID string
	Key         string
	Digest      string
	LeaseTTL    time.Duration
}
type PreRegisterResponse struct {
	RequestID string
	ID        string
	Status    string
	Created   bool
	Version   int64
}

func (s *UseCase) PreRegister(ctx context.Context, in PreRegisterRequest) (PreRegisterResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return PreRegisterResponse{}, err
	}
	if blank(in.CallerID) || blank(in.NamespaceID) || blank(in.Key) || blank(in.Digest) {
		return PreRegisterResponse{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	scope := s.keyScope(in.CallerID, in.NamespaceID, in.Key)
	if existing, ok := s.byKey[scope]; ok {
		if existing.Digest != in.Digest {
			return PreRegisterResponse{}, ErrConflict
		}
		now := s.nowFunc()
		if in.LeaseTTL > 0 && (existing.Status == StatusPreRegistered || existing.Status == StatusReleased || existing.Status == StatusFailed) {
			existing.LeaseExpiresAt = now.Add(in.LeaseTTL)
			existing.Version++
			existing.UpdatedAt = now
			_ = s.persist()
		}
		return PreRegisterResponse{RequestID: existing.ID, ID: existing.ID, Status: existing.Status, Created: false, Version: existing.Version}, nil
	}
	now := s.nowFunc()
	req := &Request{ID: s.newID("request"), CallerID: in.CallerID, NamespaceID: in.NamespaceID, Key: in.Key, Digest: in.Digest, Status: StatusPreRegistered, Version: 1, CreatedAt: now, UpdatedAt: now}
	s.requests[req.ID] = req
	s.byKey[scope] = req
	_ = s.persist()
	return PreRegisterResponse{RequestID: req.ID, ID: req.ID, Status: req.Status, Created: true, Version: req.Version}, nil
}

type OccupyRequest struct {
	RequestID       string
	ExpectedVersion int64
	LeaseTTL        time.Duration
}
type OccupyResponse struct {
	RequestID    string
	ID           string
	CredentialID string
	Generation   int64
	Status       string
	Version      int64
}

func (s *UseCase) Occupy(ctx context.Context, in OccupyRequest) (OccupyResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return OccupyResponse{}, err
	}
	if blank(in.RequestID) {
		return OccupyResponse{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[in.RequestID]
	if !ok {
		return OccupyResponse{}, ErrNotFound
	}
	if in.ExpectedVersion >= 0 && req.Version != in.ExpectedVersion {
		return OccupyResponse{}, ErrOptimisticLock
	}
	now := s.nowFunc()
	ttl := in.LeaseTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	switch req.Status {
	case StatusPreRegistered, StatusReleased, StatusFailed:
		if blank(req.Digest) {
			return OccupyResponse{}, ErrInvalid
		}
		gen := req.Generation + 1
		retainedFailure := req.FailureReason
		if req.Status == StatusReleased || req.Status == StatusFailed {
			gen, retainedFailure = retryState(req.Generation, "")
		}
		credID := s.newID("credential")
		cred := &Credential{ID: credID, RequestID: req.ID, Generation: gen, Status: credentialActive, Version: 1, ExpiresAt: now.Add(ttl), CreatedAt: now, UpdatedAt: now}
		s.credentials[credID] = cred
		req.CredentialID = credID
		req.Generation = gen
		req.Status = StatusOccupied
		req.FailureReason = retainedFailure
		req.LeaseExpiresAt = cred.ExpiresAt
		req.Version++
		req.UpdatedAt = now
		_ = s.persist()
		return OccupyResponse{RequestID: req.ID, ID: req.ID, CredentialID: credID, Generation: gen, Status: req.Status, Version: req.Version}, nil
	case StatusOccupied:
		if blank(req.CredentialID) {
			return OccupyResponse{}, ErrNotFound
		}
		cred, ok := s.credentials[req.CredentialID]
		if !ok {
			return OccupyResponse{}, ErrNotFound
		}
		if now.Before(cred.ExpiresAt) {
			return OccupyResponse{RequestID: req.ID, ID: req.ID, CredentialID: cred.ID, Generation: cred.Generation, Status: req.Status, Version: req.Version}, nil
		}
		gen := req.Generation + 1
		cred.Status = credentialExpired
		cred.Version++
		cred.UpdatedAt = now
		newCredID := s.newID("credential")
		newCred := &Credential{ID: newCredID, RequestID: req.ID, Generation: gen, Status: credentialActive, Version: 1, ExpiresAt: now.Add(ttl), CreatedAt: now, UpdatedAt: now}
		s.credentials[newCredID] = newCred
		req.CredentialID = newCredID
		req.Generation = gen
		req.Status = StatusOccupied
		req.LeaseExpiresAt = newCred.ExpiresAt
		req.Version++
		req.UpdatedAt = now
		_ = s.persist()
		return OccupyResponse{RequestID: req.ID, ID: req.ID, CredentialID: newCredID, Generation: gen, Status: req.Status, Version: req.Version}, nil
	case StatusCommitted:
		return OccupyResponse{}, ErrIllegalTransition
	default:
		return OccupyResponse{}, ErrIllegalTransition
	}
}

type CommitRequest struct {
	RequestID       string
	CredentialID    string
	Generation      int64
	Payload         map[string]any
	ExpectedVersion int64
}
type CommitResponse struct {
	RequestID string
	ID        string
	ResultID  string
	Status    string
	Version   int64
	Hit       bool
}

func (s *UseCase) Commit(ctx context.Context, in CommitRequest) (CommitResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return CommitResponse{}, err
	}
	if blank(in.RequestID) || blank(in.CredentialID) {
		return CommitResponse{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[in.RequestID]
	if !ok {
		return CommitResponse{}, ErrNotFound
	}
	if in.ExpectedVersion >= 0 && req.Version != in.ExpectedVersion {
		return CommitResponse{}, ErrOptimisticLock
	}
	cred, ok := s.credentials[in.CredentialID]
	if !ok {
		return CommitResponse{}, ErrNotFound
	}
	if cred.RequestID != req.ID {
		return CommitResponse{}, ErrConflict
	}
	now := s.nowFunc()
	if req.Status == StatusCommitted {
		if in.Generation != req.Generation {
			return CommitResponse{}, ErrConflict
		}
		if blank(req.ResultID) {
			return CommitResponse{}, ErrNotFound
		}
		res, ok := s.results[req.ResultID]
		if !ok {
			return CommitResponse{}, ErrNotFound
		}
		req.Version++
		req.UpdatedAt = now
		_ = s.persist()
		return CommitResponse{RequestID: req.ID, ID: req.ID, ResultID: res.ID, Status: req.Status, Version: req.Version, Hit: true}, nil
	}
	if req.Status != StatusOccupied {
		return CommitResponse{}, ErrIllegalTransition
	}
	if in.Generation != req.Generation {
		return CommitResponse{}, ErrConflict
	}
	if cred.Status != credentialActive {
		return CommitResponse{}, ErrIllegalTransition
	}
	if now.After(cred.ExpiresAt) {
		return CommitResponse{}, ErrExpired
	}
	resultID := s.newID("result")
	res := &Result{ID: resultID, RequestID: req.ID, Generation: req.Generation, Payload: clonePayload(in.Payload), Version: 1, CreatedAt: now, UpdatedAt: now}
	s.results[resultID] = res
	req.Status = StatusCommitted
	req.ResultID = resultID
	req.CommittedAt = now
	req.Version++
	req.UpdatedAt = now
	cred.Status = credentialCommitted
	cred.Version++
	cred.UpdatedAt = now
	_ = s.persist()
	return CommitResponse{RequestID: req.ID, ID: req.ID, ResultID: resultID, Status: req.Status, Version: req.Version, Hit: false}, nil
}

type ReplayRequest struct{ RequestID string }
type ReplayResponse struct {
	RequestID  string
	ID         string
	ResultID   string
	Generation int64
	Payload    map[string]any
	Hit        bool
}

func (s *UseCase) Replay(ctx context.Context, in ReplayRequest) (ReplayResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return ReplayResponse{}, err
	}
	if blank(in.RequestID) {
		return ReplayResponse{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[in.RequestID]
	if !ok {
		return ReplayResponse{}, ErrNotFound
	}
	if req.Status != StatusCommitted || blank(req.ResultID) {
		return ReplayResponse{}, ErrIllegalTransition
	}
	res, ok := s.results[req.ResultID]
	if !ok {
		return ReplayResponse{}, ErrNotFound
	}
	replay := Replay{ID: s.newID("replay"), RequestID: req.ID, ResultID: res.ID, Generation: res.Generation, HitAt: s.nowFunc(), Version: 1}
	s.replays[req.ID] = append(s.replays[req.ID], replay)
	req.Version++
	req.UpdatedAt = s.nowFunc()
	_ = s.persist()
	return ReplayResponse{RequestID: req.ID, ID: req.ID, ResultID: res.ID, Generation: res.Generation, Payload: clonePayload(res.Payload), Hit: true}, nil
}

type FailReleaseRequest struct {
	RequestID       string
	CredentialID    string
	Generation      int64
	Reason          string
	ExpectedVersion int64
}
type FailReleaseResponse struct {
	RequestID string
	ID        string
	Status    string
	Version   int64
	Released  bool
}

func (s *UseCase) FailRelease(ctx context.Context, in FailReleaseRequest) (FailReleaseResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return FailReleaseResponse{}, err
	}
	if blank(in.RequestID) {
		return FailReleaseResponse{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[in.RequestID]
	if !ok {
		return FailReleaseResponse{}, ErrNotFound
	}
	if in.ExpectedVersion >= 0 && req.Version != in.ExpectedVersion {
		return FailReleaseResponse{}, ErrOptimisticLock
	}
	if req.Status == StatusCommitted {
		return FailReleaseResponse{}, ErrIllegalTransition
	}
	if req.Status != StatusOccupied {
		return FailReleaseResponse{}, ErrIllegalTransition
	}
	if blank(req.CredentialID) {
		return FailReleaseResponse{}, ErrNotFound
	}
	cred, ok := s.credentials[req.CredentialID]
	if !ok {
		return FailReleaseResponse{}, ErrNotFound
	}
	if in.Generation != req.Generation {
		return FailReleaseResponse{}, ErrConflict
	}
	now := s.nowFunc()
	if now.After(cred.ExpiresAt) {
		cred.Status = credentialExpired
	} else {
		cred.Status = credentialReleased
	}
	cred.Version++
	cred.UpdatedAt = now
	req.Status = StatusReleased
	req.FailureReason = in.Reason
	req.UpdatedAt = now
	req.Version++
	if in.Reason != "" {
		s.failures[req.ID] = append(s.failures[req.ID], in.Reason)
	}
	_ = s.persist()
	return FailReleaseResponse{RequestID: req.ID, ID: req.ID, Status: req.Status, Version: req.Version, Released: true}, nil
}

type QueryRequest struct {
	CallerID    string
	NamespaceID string
	Status      string
	Key         string
	OnlyExpired bool
	Limit       int
	Offset      int
}
type QueryResponse struct {
	Items []Request
	Total int
}

func (s *UseCase) QueryRequests(ctx context.Context, q QueryRequest) (QueryResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return QueryResponse{}, err
	}
	s.mu.RLock()
	now := time.Now()
	items := []Request{}
	for _, req := range s.requests {
		r := cloneRequest(req)
		if q.CallerID != "" && r.CallerID != q.CallerID {
			continue
		}
		if q.NamespaceID != "" && r.NamespaceID != q.NamespaceID {
			continue
		}
		if q.Status != "" && r.Status != q.Status {
			continue
		}
		if q.Key != "" && r.Key != q.Key {
			continue
		}
		if q.OnlyExpired && !now.After(r.LeaseExpiresAt) {
			continue
		}
		items = append(items, r)
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		wi := now.Sub(items[i].CreatedAt)
		wj := now.Sub(items[j].CreatedAt)
		if wi != wj {
			return wi > wj
		}
		if items[i].CallerID != items[j].CallerID {
			return items[i].CallerID < items[j].CallerID
		}
		if items[i].Key != items[j].Key {
			return items[i].Key < items[j].Key
		}
		return items[i].ID < items[j].ID
	})
	total := len(items)
	if q.Offset > 0 {
		if q.Offset >= total {
			items = []Request{}
		} else {
			items = items[q.Offset:]
		}
	}
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[:q.Limit]
	}
	return QueryResponse{Items: items, Total: total}, nil
}
func (s *UseCase) ListHangingExecutions(ctx context.Context) (QueryResponse, error) {
	return s.QueryRequests(ctx, QueryRequest{Status: StatusOccupied, OnlyExpired: true})
}
func (s *UseCase) ListConflictingRequests(ctx context.Context) (QueryResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return QueryResponse{}, err
	}
	s.mu.RLock()
	items := []Request{}
	for _, req := range s.requests {
		if req.Status == StatusOccupied || req.Status == StatusCommitted {
			items = append(items, cloneRequest(req))
		}
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].CallerID != items[j].CallerID {
			return items[i].CallerID < items[j].CallerID
		}
		if items[i].Key != items[j].Key {
			return items[i].Key < items[j].Key
		}
		return items[i].ID < items[j].ID
	})
	return QueryResponse{Items: items, Total: len(items)}, nil
}
func (s *UseCase) FindRequest(ctx context.Context, id string) (Request, error) {
	if err := s.checkContext(ctx); err != nil {
		return Request{}, err
	}
	if blank(id) {
		return Request{}, ErrInvalid
	}
	s.mu.RLock()
	req, ok := s.requests[id]
	out := cloneRequest(req)
	s.mu.RUnlock()
	if !ok {
		return Request{}, ErrNotFound
	}
	return out, nil
}

type ReplayHitRateResponse struct {
	Committed int
	Replays   int
	Rate      float64
}

func (s *UseCase) ReplayHitRate(ctx context.Context) (ReplayHitRateResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return ReplayHitRateResponse{}, err
	}
	s.mu.RLock()
	committed := 0
	replays := 0
	for _, req := range s.requests {
		if req.Status == StatusCommitted {
			committed++
		}
	}
	for _, list := range s.replays {
		replays += len(list)
	}
	s.mu.RUnlock()
	total := committed + replays
	rate := 0.0
	if total > 0 {
		rate = float64(replays) / float64(total)
	}
	return ReplayHitRateResponse{Committed: committed, Replays: replays, Rate: rate}, nil
}

type BatchItemResult struct {
	Index   int
	ID      string
	Success bool
	Error   string
}

func (s *UseCase) PreRegisterBatch(ctx context.Context, items []PreRegisterRequest) []BatchItemResult {
	results := make([]BatchItemResult, len(items))
	for i, item := range items {
		if err := s.checkContext(ctx); err != nil {
			results[i] = BatchItemResult{Index: i, Error: err.Error()}
			continue
		}
		resp, err := s.PreRegister(ctx, item)
		if err != nil {
			results[i] = BatchItemResult{Index: i, Error: err.Error()}
			continue
		}
		results[i] = BatchItemResult{Index: i, ID: resp.RequestID, Success: true}
	}
	return results
}
func (s *UseCase) OccupyBatch(ctx context.Context, items []OccupyRequest) []BatchItemResult {
	results := make([]BatchItemResult, len(items))
	for i, item := range items {
		if err := s.checkContext(ctx); err != nil {
			results[i] = BatchItemResult{Index: i, Error: err.Error()}
			continue
		}
		resp, err := s.Occupy(ctx, item)
		if err != nil {
			results[i] = BatchItemResult{Index: i, Error: err.Error()}
			continue
		}
		results[i] = BatchItemResult{Index: i, ID: resp.RequestID, Success: true}
	}
	return results
}
func (s *UseCase) CommitBatch(ctx context.Context, items []CommitRequest) []BatchItemResult {
	results := make([]BatchItemResult, len(items))
	for i, item := range items {
		if err := s.checkContext(ctx); err != nil {
			results[i] = BatchItemResult{Index: i, Error: err.Error()}
			continue
		}
		resp, err := s.Commit(ctx, item)
		if err != nil {
			results[i] = BatchItemResult{Index: i, Error: err.Error()}
			continue
		}
		results[i] = BatchItemResult{Index: i, ID: resp.RequestID, Success: true}
	}
	return results
}

func (s *UseCase) SelfCheck(ctx context.Context) error {
	if err := s.checkContext(ctx); err != nil {
		return err
	}
	key := s.newID("self-check")
	pr, err := s.PreRegister(ctx, PreRegisterRequest{CallerID: "self", NamespaceID: "default", Key: key, Digest: "sha256:" + key, LeaseTTL: time.Minute})
	if err != nil {
		return err
	}
	oc, err := s.Occupy(ctx, OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: time.Minute})
	if err != nil {
		return err
	}
	cm, err := s.Commit(ctx, CommitRequest{RequestID: pr.RequestID, CredentialID: oc.CredentialID, Generation: oc.Generation, ExpectedVersion: oc.Version, Payload: map[string]any{"ok": true}})
	if err != nil {
		return err
	}
	rp, err := s.Replay(ctx, ReplayRequest{RequestID: cm.RequestID})
	if err != nil {
		return err
	}
	if !rp.Hit {
		return ErrInvalid
	}
	q, err := s.QueryRequests(ctx, QueryRequest{CallerID: "self", Status: StatusCommitted})
	if err != nil {
		return err
	}
	if q.Total == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *UseCase) RunSelfCheck(ctx context.Context) error { return s.SelfCheck(ctx) }
func (s *UseCase) Run(ctx context.Context) error          { return s.SelfCheck(ctx) }
func (s *UseCase) Start(ctx context.Context) error        { return s.SelfCheck(ctx) }
