// Package session owns bounded, in-memory room credentials and private wake
// registration. It is transport-independent and never grants a speaking turn.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvite       = errors.New("invalid or expired invitation")
	ErrFull         = errors.New("room capacity reached")
	ErrCapacity     = errors.New("service capacity reached")
	ErrMetadata     = errors.New("invalid participant metadata")
	ErrWake         = errors.New("invalid or unavailable wake provider")
	ErrEntropy      = errors.New("credential generation failed")
)

const (
	MaxMembers      = 4
	InviteLifetime  = 15 * time.Minute
	SessionLifetime = 24 * time.Hour
)

type Metadata struct {
	DisplayName, Platform, AppVersion string
	Capabilities                      []string
}
type Member struct {
	ParticipantID, DisplayName, Platform, Reachability string
	ControlConnected                                   bool
}
type View struct {
	RoomID, Epoch string
	Members       []Member
}
type Issued struct {
	View
	ParticipantID                        string
	MemberCredential                     string `json:"-"`
	AdminCredential                      string `json:"-"`
	Invite                               string `json:"-"`
	CredentialExpiresAt, InviteExpiresAt time.Time
}
type Actor struct {
	RoomID, Epoch, ParticipantID string
	ExpiresAt                    time.Time
}
type Connection struct {
	Actor
	Generation uint64
}
type WakeRegistration struct {
	Provider, ChannelID, Environment string
	Token                            string `json:"-"`
}
type WakeTarget struct {
	RoomID, Epoch, ParticipantID string
	Generation                   uint64
	Registration                 WakeRegistration
}
type Config struct {
	MaxRooms        int
	APNsEnvironment string
	APNsEnabled     bool
	Now             func() time.Time
}
type tokenRef struct {
	room, member string
	admin        bool
}
type participant struct {
	id             string
	meta           Metadata
	hash           [32]byte
	online         bool
	connection     uint64
	wake           WakeRegistration
	wakeGeneration uint64
}
type room struct {
	id, epoch             string
	expires               time.Time
	adminHash, inviteHash [32]byte
	inviteExpires         time.Time
	members               map[string]*participant
}
type Store struct {
	mu     sync.Mutex
	cfg    Config
	random io.Reader
	rooms  map[string]*room
	tokens map[[32]byte]tokenRef
}

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func New(cfg Config) (*Store, error) {
	if cfg.MaxRooms == 0 {
		cfg.MaxRooms = 128
	}
	if cfg.MaxRooms < 1 || cfg.MaxRooms > 10000 {
		return nil, ErrCapacity
	}
	if cfg.APNsEnabled && cfg.APNsEnvironment != "sandbox" && cfg.APNsEnvironment != "production" {
		return nil, ErrWake
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Store{cfg: cfg, random: rand.Reader, rooms: map[string]*room{}, tokens: map[[32]byte]tokenRef{}}, nil
}
func valid(m Metadata) bool {
	if !utf8.ValidString(m.DisplayName) || utf8.RuneCountInString(m.DisplayName) < 1 || utf8.RuneCountInString(m.DisplayName) > 48 || !utf8.ValidString(m.AppVersion) || utf8.RuneCountInString(m.AppVersion) < 1 || utf8.RuneCountInString(m.AppVersion) > 64 || len(m.Capabilities) > 16 {
		return false
	}
	required := ""
	switch m.Platform {
	case "android":
		required = "android-joined-service-v1"
	case "ios":
		required = "ptt-apns-v1"
	default:
		return false
	}
	seen := map[string]bool{}
	for _, c := range m.Capabilities {
		if seen[c] {
			return false
		}
		switch c {
		case "lan-webrtc-v1", "android-joined-service-v1", "ptt-apns-v1":
		default:
			return false
		}
		seen[c] = true
	}
	return seen["lan-webrtc-v1"] && seen[required]
}
func clone(m Metadata) Metadata { m.Capabilities = append([]string(nil), m.Capabilities...); return m }

// mint rejects collisions before publishing any partial state. Raw credentials
// are returned once; only SHA-256 digests are retained for authentication.
func (s *Store) mint(count int) ([]string, error) {
	values := []string{}
	seen := map[string]bool{}
	for tries := 0; len(values) < count && tries < count*8; tries++ {
		b := make([]byte, 32)
		if _, e := io.ReadFull(s.random, b); e != nil {
			return nil, ErrEntropy
		}
		v := base64.RawURLEncoding.EncodeToString(b)
		if seen[v] || s.used(v) {
			continue
		}
		seen[v] = true
		values = append(values, v)
	}
	if len(values) != count {
		return nil, ErrEntropy
	}
	return values, nil
}

// used includes non-authentication identifiers and invitation digests, so a
// defective entropy source cannot overwrite a member or preserve a rotated invite.
func (s *Store) used(v string) bool {
	hash := sha256.Sum256([]byte(v))
	if _, ok := s.tokens[hash]; ok {
		return true
	}
	for _, r := range s.rooms {
		if r.id == v || r.epoch == v || r.inviteHash == hash || r.members[v] != nil {
			return true
		}
	}
	return false
}
func (s *Store) drop(r *room) {
	for _, m := range r.members {
		delete(s.tokens, m.hash)
	}
	delete(s.tokens, r.adminHash)
	delete(s.rooms, r.id)
}
func (s *Store) prune(now time.Time) {
	for _, r := range s.rooms {
		if !now.Before(r.expires) {
			s.drop(r)
		}
	}
}
func (s *Store) Create(meta Metadata) (Issued, error) {
	if !valid(meta) {
		return Issued{}, ErrMetadata
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.cfg.Now()
	s.prune(now)
	if len(s.rooms) >= s.cfg.MaxRooms {
		return Issued{}, ErrCapacity
	}
	v, e := s.mint(6)
	if e != nil {
		return Issued{}, e
	}
	r := &room{id: v[0], epoch: v[1], expires: now.Add(SessionLifetime), adminHash: sha256.Sum256([]byte(v[4])), inviteHash: sha256.Sum256([]byte(v[5])), inviteExpires: now.Add(InviteLifetime), members: map[string]*participant{}}
	m := &participant{id: v[2], meta: clone(meta), hash: sha256.Sum256([]byte(v[3])), wake: WakeRegistration{Provider: "none"}}
	r.members[m.id] = m
	s.rooms[r.id] = r
	s.tokens[m.hash] = tokenRef{room: r.id, member: m.id}
	s.tokens[r.adminHash] = tokenRef{room: r.id, admin: true}
	return Issued{View: view(r), ParticipantID: m.id, MemberCredential: v[3], AdminCredential: v[4], Invite: v[5], CredentialExpiresAt: r.expires, InviteExpiresAt: r.inviteExpires}, nil
}
func (s *Store) Join(roomID, invite string, meta Metadata) (Issued, error) {
	if !valid(meta) {
		return Issued{}, ErrMetadata
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.cfg.Now()
	s.prune(now)
	r := s.rooms[roomID]
	if r == nil || len(invite) != 43 || !now.Before(r.inviteExpires) || sha256.Sum256([]byte(invite)) != r.inviteHash {
		return Issued{}, ErrInvite
	}
	if len(r.members) >= MaxMembers {
		return Issued{}, ErrFull
	}
	v, e := s.mint(2)
	if e != nil {
		return Issued{}, e
	}
	m := &participant{id: v[0], meta: clone(meta), hash: sha256.Sum256([]byte(v[1])), wake: WakeRegistration{Provider: "none"}}
	r.members[m.id] = m
	s.tokens[m.hash] = tokenRef{room: r.id, member: m.id}
	return Issued{View: view(r), ParticipantID: m.id, MemberCredential: v[1], CredentialExpiresAt: r.expires}, nil
}
func (s *Store) auth(roomID, token string, admin bool) (*room, *participant, error) {
	if len(token) != 43 {
		return nil, nil, ErrUnauthorized
	}
	ref, ok := s.tokens[sha256.Sum256([]byte(token))]
	if !ok || ref.room != roomID || ref.admin != admin {
		return nil, nil, ErrUnauthorized
	}
	r := s.rooms[roomID]
	if r == nil {
		return nil, nil, ErrUnauthorized
	}
	if admin {
		return r, nil, nil
	}
	m := r.members[ref.member]
	if m == nil {
		return nil, nil, ErrUnauthorized
	}
	return r, m, nil
}
func actor(r *room, m *participant) Actor {
	return Actor{RoomID: r.id, Epoch: r.epoch, ParticipantID: m.id, ExpiresAt: r.expires}
}
func (s *Store) Authenticate(roomID, token string) (Actor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r, m, e := s.auth(roomID, token, false)
	if e != nil {
		return Actor{}, e
	}
	return actor(r, m), nil
}
func view(r *room) View {
	v := View{RoomID: r.id, Epoch: r.epoch, Members: []Member{}}
	for _, m := range r.members {
		reach := "unreachable"
		if m.online {
			reach = "online"
		} else if m.wake.Provider == "apns-ptt" {
			reach = "wakeable"
		}
		v.Members = append(v.Members, Member{m.id, m.meta.DisplayName, m.meta.Platform, reach, m.online})
	}
	sort.Slice(v.Members, func(i, j int) bool { return v.Members[i].ParticipantID < v.Members[j].ParticipantID })
	return v
}
func (s *Store) Snapshot(roomID, token string) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r, _, e := s.auth(roomID, token, false)
	if e != nil {
		return View{}, e
	}
	return view(r), nil
}
func (s *Store) Connect(roomID, token string) (Connection, View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r, m, e := s.auth(roomID, token, false)
	if e != nil {
		return Connection{}, View{}, e
	}
	m.connection++
	m.online = true
	return Connection{actor(r, m), m.connection}, view(r), nil
}

// IsCurrent must gate processing from a socket after it is replaced by reconnect.
func (s *Store) IsCurrent(c Connection) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r := s.rooms[c.RoomID]
	if r == nil || r.epoch != c.Epoch {
		return false
	}
	m := r.members[c.ParticipantID]
	return m != nil && m.online && m.connection == c.Generation
}

// Disconnect returns true only if it actually disconnected the current socket.
// The coordinator must not revoke a floor for an ignored old-socket callback.
func (s *Store) Disconnect(c Connection) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r := s.rooms[c.RoomID]
	if r == nil || r.epoch != c.Epoch {
		return false
	}
	m := r.members[c.ParticipantID]
	if m == nil || !m.online || m.connection != c.Generation {
		return false
	}
	m.online = false
	return true
}
func (s *Store) Leave(roomID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r, m, e := s.auth(roomID, token, false)
	if e != nil {
		return e
	}
	delete(r.members, m.id)
	delete(s.tokens, m.hash)
	return nil
}
func (s *Store) Remove(roomID, adminToken, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r, _, e := s.auth(roomID, adminToken, true)
	if e != nil {
		return e
	}
	if m := r.members[memberID]; m != nil {
		delete(s.tokens, m.hash)
		delete(r.members, memberID)
	}
	return nil
}
func (s *Store) Close(roomID, adminToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r, _, e := s.auth(roomID, adminToken, true)
	if e != nil {
		return e
	}
	s.drop(r)
	return nil
}
func (s *Store) RotateInvite(roomID, adminToken string) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.cfg.Now()
	s.prune(now)
	r, _, e := s.auth(roomID, adminToken, true)
	if e != nil {
		return "", time.Time{}, e
	}
	v, e := s.mint(1)
	if e != nil {
		return "", time.Time{}, e
	}
	r.inviteHash = sha256.Sum256([]byte(v[0]))
	r.inviteExpires = now.Add(InviteLifetime)
	if r.inviteExpires.After(r.expires) {
		r.inviteExpires = r.expires
	}
	return v[0], r.inviteExpires, nil
}
func (s *Store) RegisterWake(roomID, token string, w WakeRegistration) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	_, m, e := s.auth(roomID, token, false)
	if e != nil {
		return 0, e
	}
	switch w.Provider {
	case "none":
		if w.Token != "" || w.ChannelID != "" || w.Environment != "" {
			return 0, ErrWake
		}
	case "apns-ptt":
		if !s.cfg.APNsEnabled || m.meta.Platform != "ios" || w.Environment != s.cfg.APNsEnvironment || !uuid.MatchString(w.ChannelID) || len(w.Token) < 2 || len(w.Token) > 1024 {
			return 0, ErrWake
		}
		if _, e := hex.DecodeString(w.Token); e != nil {
			return 0, ErrWake
		}
	default:
		return 0, ErrWake
	}
	m.wakeGeneration++
	m.wake = w
	return m.wakeGeneration, nil
}

// WakeTargetFor is for trusted coordinator jobs only, not an unauthenticated API.
func (s *Store) WakeTargetFor(roomID, memberID string) (WakeTarget, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r := s.rooms[roomID]
	if r == nil {
		return WakeTarget{}, false
	}
	m := r.members[memberID]
	if m == nil || m.wake.Provider != "apns-ptt" {
		return WakeTarget{}, false
	}
	return WakeTarget{r.id, r.epoch, m.id, m.wakeGeneration, m.wake}, true
}
func (s *Store) InvalidateWake(target WakeTarget) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.cfg.Now())
	r := s.rooms[target.RoomID]
	if r == nil || r.epoch != target.Epoch {
		return false
	}
	m := r.members[target.ParticipantID]
	if m == nil || m.wakeGeneration != target.Generation || m.wake.Provider != "apns-ptt" {
		return false
	}
	m.wakeGeneration++
	m.wake = WakeRegistration{Provider: "none"}
	return true
}
