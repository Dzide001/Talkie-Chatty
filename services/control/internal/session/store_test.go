package session

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func metadata(platform string) Metadata {
	capability := "android-joined-service-v1"
	if platform == "ios" {
		capability = "ptt-apns-v1"
	}
	return Metadata{"Listener", platform, "0.1", []string{"lan-webrtc-v1", capability}}
}
func fixture(t *testing.T, enabled bool) (*Store, Issued, *time.Time) {
	t.Helper()
	now := time.Unix(1000, 0)
	s, err := New(Config{APNsEnabled: enabled, APNsEnvironment: "sandbox", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := s.Create(metadata("ios"))
	if err != nil {
		t.Fatal(err)
	}
	return s, issued, &now
}
func TestCredentialIsolation(t *testing.T) {
	s, a, _ := fixture(t, false)
	b, err := s.Create(metadata("android"))
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ room, token string }{{a.RoomID, a.AdminCredential}, {b.RoomID, a.MemberCredential}, {a.RoomID, a.Invite}, {a.RoomID, "bad"}} {
		if _, err := s.Authenticate(check.room, check.token); err != ErrUnauthorized {
			t.Fatalf("accepted wrong credential: %v", err)
		}
	}
	if err := s.Close(a.RoomID, a.MemberCredential); err != ErrUnauthorized {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(a.RoomID, a.MemberCredential); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{a.MemberCredential, a.AdminCredential, a.Invite} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("JSON leaked secret")
		}
	}
	if err := s.Close(a.RoomID, a.AdminCredential); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(a.RoomID, a.MemberCredential); err != ErrUnauthorized {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(b.RoomID, b.MemberCredential); err != nil {
		t.Fatal(err)
	}
}
func TestInviteRotationAndExpiry(t *testing.T) {
	s, a, now := fixture(t, false)
	invite, expires, err := s.RotateInvite(a.RoomID, a.AdminCredential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Join(a.RoomID, a.Invite, metadata("android")); err != ErrInvite {
		t.Fatal(err)
	}
	joined, err := s.Join(a.RoomID, invite, metadata("android"))
	if err != nil {
		t.Fatal(err)
	}
	if joined.AdminCredential != "" || joined.Invite != "" {
		t.Fatal("join returned admin authority")
	}
	*now = expires
	if _, err := s.Join(a.RoomID, invite, metadata("android")); err != ErrInvite {
		t.Fatal(err)
	}
	*now = a.CredentialExpiresAt.Add(-time.Minute)
	_, expires, err = s.RotateInvite(a.RoomID, a.AdminCredential)
	if err != nil || !expires.Equal(a.CredentialExpiresAt) {
		t.Fatal("invite outlives room", err)
	}
	*now = a.CredentialExpiresAt
	if _, err := s.Authenticate(a.RoomID, joined.MemberCredential); err != ErrUnauthorized {
		t.Fatal(err)
	}
	if len(s.rooms) != 0 || len(s.tokens) != 0 {
		t.Fatal("expired credentials retained")
	}
}
func TestConcurrentCapacity(t *testing.T) {
	s, a, _ := fixture(t, false)
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Join(a.RoomID, a.Invite, metadata("android")); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if err != ErrFull {
			t.Fatal(err)
		}
	}
	if successes != 3 {
		t.Fatalf("joined %d", successes)
	}
	limited, _ := New(Config{MaxRooms: 1})
	if _, err := limited.Create(metadata("android")); err != nil {
		t.Fatal(err)
	}
	if _, err := limited.Create(metadata("android")); err != ErrCapacity {
		t.Fatal(err)
	}
}
func TestReconnectGenerationAndSnapshotIsolation(t *testing.T) {
	s, a, _ := fixture(t, false)
	old, _, err := s.Connect(a.RoomID, a.MemberCredential)
	if err != nil {
		t.Fatal(err)
	}
	current, v, err := s.Connect(a.RoomID, a.MemberCredential)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsCurrent(old) || s.Disconnect(old) || !s.IsCurrent(current) {
		t.Fatal("old socket replaced current connection")
	}
	v.Members[0].DisplayName = "mutated"
	next, _ := s.Snapshot(a.RoomID, a.MemberCredential)
	if next.Members[0].DisplayName == "mutated" || next.Members[0].Reachability != "online" {
		t.Fatal(next)
	}
	if !s.Disconnect(current) || s.Disconnect(current) || s.IsCurrent(current) {
		t.Fatal("disconnect not idempotent")
	}
	next, _ = s.Snapshot(a.RoomID, a.MemberCredential)
	if next.Members[0].Reachability != "unreachable" {
		t.Fatal(next)
	}
}
func registration() WakeRegistration {
	return WakeRegistration{"apns-ptt", "12345678-1234-1234-1234-123456789abc", "sandbox", strings.Repeat("ab", 32)}
}
func TestWakeValidationAndPrivateStorage(t *testing.T) {
	s, a, _ := fixture(t, true)
	w := registration()
	bad := []WakeRegistration{{Provider: "fcm"}, {Provider: "none", Token: "ab"}, w, w, w}
	bad[2].Environment = "production"
	bad[3].Token = "odd"
	bad[4].ChannelID = "bad"
	for _, w := range bad {
		if _, err := s.RegisterWake(a.RoomID, a.MemberCredential, w); err != ErrWake {
			t.Fatalf("invalid wake accepted: %v", err)
		}
	}
	if _, err := s.RegisterWake(a.RoomID, a.MemberCredential, w); err != nil {
		t.Fatal(err)
	}
	v, _ := s.Snapshot(a.RoomID, a.MemberCredential)
	if v.Members[0].Reachability != "wakeable" {
		t.Fatal(v)
	}
	target, ok := s.WakeTargetFor(a.RoomID, a.ParticipantID)
	if !ok || target.Registration.Token != w.Token {
		t.Fatal("private lookup failed")
	}
	data, _ := json.Marshal(target)
	if bytes.Contains(data, []byte(w.Token)) {
		t.Fatal("wake token exposed in JSON")
	}
	android, err := s.Join(a.RoomID, a.Invite, metadata("android"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegisterWake(a.RoomID, android.MemberCredential, w); err != ErrWake {
		t.Fatal(err)
	}
	disabled, b, _ := fixture(t, false)
	if _, err := disabled.RegisterWake(b.RoomID, b.MemberCredential, w); err != ErrWake {
		t.Fatal(err)
	}
}
func TestStaleWakeInvalidationAndMemberRemoval(t *testing.T) {
	s, a, _ := fixture(t, true)
	if _, err := s.RegisterWake(a.RoomID, a.MemberCredential, registration()); err != nil {
		t.Fatal(err)
	}
	old, _ := s.WakeTargetFor(a.RoomID, a.ParticipantID)
	w := registration()
	w.Token = strings.Repeat("cd", 32)
	if _, err := s.RegisterWake(a.RoomID, a.MemberCredential, w); err != nil {
		t.Fatal(err)
	}
	if s.InvalidateWake(old) {
		t.Fatal("stale APNs failure invalidated replacement")
	}
	current, _ := s.WakeTargetFor(a.RoomID, a.ParticipantID)
	if !s.InvalidateWake(current) || s.InvalidateWake(current) {
		t.Fatal("invalidation not idempotent")
	}
	if _, err := s.RegisterWake(a.RoomID, a.MemberCredential, w); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(a.RoomID, a.AdminCredential, a.ParticipantID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.WakeTargetFor(a.RoomID, a.ParticipantID); ok {
		t.Fatal("removed token retained")
	}
	if _, err := s.Authenticate(a.RoomID, a.MemberCredential); err != ErrUnauthorized {
		t.Fatal(err)
	}
	if err := s.Close(a.RoomID, a.AdminCredential); err != nil {
		t.Fatal("admin lost on member removal", err)
	}
}
func TestLeaveInvalidatesConnection(t *testing.T) {
	s, a, _ := fixture(t, false)
	c, _, _ := s.Connect(a.RoomID, a.MemberCredential)
	if err := s.Leave(a.RoomID, a.MemberCredential); err != nil {
		t.Fatal(err)
	}
	if s.IsCurrent(c) || s.Disconnect(c) {
		t.Fatal("departed connection still valid")
	}
	if _, err := s.Authenticate(a.RoomID, a.MemberCredential); err != ErrUnauthorized {
		t.Fatal(err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("private entropy error") }
func TestEntropyFailureDoesNotPublishState(t *testing.T) {
	s, _ := New(Config{})
	for _, reader := range []interface{ Read([]byte) (int, error) }{failingReader{}, bytes.NewReader(make([]byte, 32*48))} {
		s.random = reader
		if _, err := s.Create(metadata("android")); err != ErrEntropy {
			t.Fatal(err)
		}
		if len(s.rooms) != 0 || len(s.tokens) != 0 {
			t.Fatal("partial state published")
		}
	}
}
func TestMetadataValidation(t *testing.T) {
	variants := []Metadata{metadata("android"), metadata("android"), metadata("android"), metadata("android"), metadata("android")}
	variants[0].DisplayName = ""
	variants[1].Platform = "web"
	variants[2].Capabilities = []string{"lan-webrtc-v1"}
	variants[3].Capabilities = append(variants[3].Capabilities, "lan-webrtc-v1")
	variants[4].AppVersion = string([]byte{0xff})
	s, _ := New(Config{})
	for _, m := range variants {
		if _, err := s.Create(m); err != ErrMetadata {
			t.Fatal(err)
		}
	}
}

func TestExistingIdentifierCollisionRejected(t *testing.T) {
	s, a, _ := fixture(t, false)
	raw, err := base64.RawURLEncoding.DecodeString(a.Invite)
	if err != nil {
		t.Fatal(err)
	}
	s.random = bytes.NewReader(bytes.Repeat(raw, 8))
	if _, _, err := s.RotateInvite(a.RoomID, a.AdminCredential); err != ErrEntropy {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(a.RoomID, a.MemberCredential); err != nil {
		t.Fatal(err)
	}
}
func TestRestartAndExpiryInvalidateConnections(t *testing.T) {
	s, a, now := fixture(t, true)
	c, _, _ := s.Connect(a.RoomID, a.MemberCredential)
	restarted, _ := New(Config{})
	if _, err := restarted.Authenticate(a.RoomID, a.MemberCredential); err != ErrUnauthorized {
		t.Fatal(err)
	}
	if restarted.IsCurrent(c) {
		t.Fatal("connection survived restart")
	}
	*now = a.CredentialExpiresAt
	if s.IsCurrent(c) || s.Disconnect(c) {
		t.Fatal("expired connection remained current")
	}
}
