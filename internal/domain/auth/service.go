package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/user"
)

var (
	// ErrSignupClosed is returned by RegisterWithPolicy when the instance does
	// not accept public registration and the users table is not empty.
	ErrSignupClosed = errors.New("signup is disabled on this instance")
	// ErrAlreadyTaken is the only client-facing conflict of registration: the
	// email or username is already in use. Every other create failure is
	// propagated as-is (an internal error for the handler to log, not echo).
	ErrAlreadyTaken = errors.New("email or username already taken")
	// ErrBootstrapRaced is returned when the first-user bootstrap lost a race
	// against another replica: the registration was undone and the client
	// should simply retry (it will then follow the normal signup policy).
	ErrBootstrapRaced = errors.New("registration raced the instance bootstrap, retry")
	// ErrInvalidCredentials is returned by Login for an unknown email or a
	// wrong password (indistinguishable on purpose).
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type Service struct {
	DB     *gorm.DB
	Secret string

	// Now is the clock used for API token expiry (creation and verification).
	// nil means time.Now; tests override it to pin "expired" vs "live".
	Now func() time.Time

	// bootstrapMu serializes RegisterWithPolicy within this process so that two
	// concurrent first registrations cannot both observe an empty users table
	// (READ COMMITTED on Postgres/MySQL would otherwise let both become admin).
	bootstrapMu sync.Mutex
	// bootstrapHook is a test seam, invoked between the user count and the
	// insert, to simulate a writer (e.g. another replica) committing in the
	// window. nil in production.
	bootstrapHook func()
}

func NewService(db *gorm.DB, secret string) *Service {
	return &Service{DB: db, Secret: secret}
}

func generateID() string {
	return uuid.New().String()
}

// Register creates a user unconditionally (no signup policy, never admin).
// It is the primitive used by seeding and tests; the HTTP register endpoint
// goes through RegisterWithPolicy.
func (s *Service) Register(email, username, displayName, password string) (*user.User, error) {
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return s.insert(email, username, displayName, hashed, false)
}

// RegisterWithPolicy creates a user honoring the instance signup policy, with
// one exception: when the instance has NO ACTIVE ADMIN (fresh install, or an
// instance whose only admin was deactivated / lost to an earlier race) the
// registration is always accepted and that user becomes global admin
// (bootstrap). Otherwise a closed policy yields ErrSignupClosed.
//
// The condition is "no active admin", not "no users": a regular user that
// landed first must never leave the instance without an admin and without a
// way to get one — and a zero-admin instance self-heals on the next
// registration, with the same trust model as a fresh one.
//
// Race handling: the count+insert is serialized in-process by bootstrapMu
// (the bcrypt hash is computed before taking it, so registrations are not
// serialized behind each other's hashing), and after a bootstrap insert the
// active-admin count is re-checked. If it is not exactly 1 another replica
// bootstrapped in the same instant: the grant is withdrawn symmetrically for
// both policies — this row is deleted and ErrSignupClosed (closed) or
// ErrBootstrapRaced (open, retryable) is returned. Deleting — rather than
// demoting — the loser matters when both replicas lose (each sees the
// other's admin): demotion would leave zero admins, whereas deletion brings
// the active-admin count back to 0 so the retry bootstraps cleanly. A regular
// user committed by another replica in the window does not touch the admin
// count, so the bootstrapper keeps its grant. Fail-safe direction: never two
// admins, never a stranded zero-admin instance.
func (s *Service) RegisterWithPolicy(email, username, displayName, password string, signupOpen bool) (*user.User, error) {
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	s.bootstrapMu.Lock()
	defer s.bootstrapMu.Unlock()

	admins, err := s.countActiveAdmins()
	if err != nil {
		return nil, err
	}
	if s.bootstrapHook != nil {
		s.bootstrapHook()
	}
	if admins > 0 {
		if !signupOpen {
			return nil, ErrSignupClosed
		}
		return s.insert(email, username, displayName, hashed, false)
	}

	// Bootstrap path: no active admin → this user becomes admin.
	u, err := s.insert(email, username, displayName, hashed, true)
	if err != nil {
		return nil, err
	}
	after, err := s.countActiveAdmins()
	if err != nil {
		return nil, err
	}
	if after == 1 {
		return u, nil
	}
	// Another admin got in between the count and the insert: withdraw the grant.
	if err := s.DB.Delete(&user.User{}, "id = ?", u.ID).Error; err != nil {
		return nil, err
	}
	if !signupOpen {
		return nil, ErrSignupClosed
	}
	return nil, ErrBootstrapRaced
}

// countActiveAdmins is the bootstrap predicate: the instance needs a
// bootstrap exactly when this is 0.
func (s *Service) countActiveAdmins() (int64, error) {
	var n int64
	if err := s.DB.Model(&user.User{}).Where("is_admin = ? AND is_active = ?", true, true).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// insert writes the users row for an already-hashed password.
func (s *Service) insert(email, username, displayName, hashed string, isAdmin bool) (*user.User, error) {
	u := &user.User{
		Email: email, Username: username, DisplayName: displayName,
		PasswordHash: hashed, IsActive: true, IsAdmin: isAdmin,
	}
	u.ID = generateID()
	if err := s.DB.Create(u).Error; err != nil {
		// Distinguish the one expected conflict from a real failure without
		// parsing driver-specific messages: if a row with that email or
		// username now exists, the insert lost to it.
		if taken, qerr := s.identityTaken(email, username); qerr == nil && taken {
			return nil, ErrAlreadyTaken
		}
		return nil, err
	}
	return u, nil
}

func (s *Service) identityTaken(email, username string) (bool, error) {
	var n int64
	err := s.DB.Model(&user.User{}).Where("email = ? OR username = ?", email, username).Count(&n).Error
	return n > 0, err
}

func (s *Service) Login(email, password string) (string, error) {
	var u user.User
	if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}
	if !VerifyPassword(u.PasswordHash, password) {
		return "", ErrInvalidCredentials
	}
	return GenerateToken(s.Secret, u.ID, 24*time.Hour)
}
