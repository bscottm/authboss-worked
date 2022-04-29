package userdb

import (
	"time"

	"github.com/volatiletech/authboss/v3"
)

// User struct for authboss
type User struct {
	authboss.User

	// User's GUID, used to map to the user's e-mail address. References the user
	// using a relatively unique value rather than by their e-mail.
	GUID string
	// User profile data:
	Name string

	// Auth
	Email    string
	Password string

	// Confirm
	ConfirmSelector string
	ConfirmVerifier string
	Confirmed       bool

	// Lock
	AttemptCount int
	LastAttempt  time.Time
	Locked       time.Time

	// Recover
	RecoverSelector    string
	RecoverVerifier    string
	RecoverTokenExpiry time.Time

	// OAuth2
	OAuth2UID          string
	OAuth2Provider     string
	OAuth2AccessToken  string
	OAuth2RefreshToken string
	OAuth2Expiry       time.Time

	// 2fa
	TOTPSecretKey      string
	SMSPhoneNumber     string
	SMSSeedPhoneNumber string
	RecoveryCodes      string

	// Remember is in another table
}

// GetPID returns the user's identifier (GUID), interface function for authboss.User
func (user *User) GetPID() string {
	return user.GUID
}

// PutPID stores the user's identifier in the User structure, interface function for authboss.User
func (user *User) PutPID(pid string) {
	user.GUID = pid
}

// GetPassword returns the bcrypt-ed user password
func (user *User) GetPassword() string {
	return user.Password
}

// PutPassword stores the bcrypt-ed user password
func (user *User) PutPassword(pass string) {
	user.Password = pass
}

// GetEmail returns the user's e-mail address
func (user *User) GetEmail() (email string) {
	return user.Email
}

// PutEmail stores the user's e-mail address
func (user *User) PutEmail(email string) {
	user.Email = email
}

// GetConfirmed returns the user's confirmation status
func (user *User) GetConfirmed() (confirmed bool) {
	return user.Confirmed
}

// PutConfirmed stores the user's confirmation status
func (user *User) PutConfirmed(confirmed bool) {
	user.Confirmed = confirmed
}

// GetConfirmSelector returns the user's confirmation selector (URL)
func (user *User) GetConfirmSelector() string {
	return user.ConfirmSelector
}

// PutConfirmSelector stores the user's confirmation selector
func (user *User) PutConfirmSelector(selector string) {
	user.ConfirmSelector = selector
}

// GetConfirmVerifier returns the user's confirmation verifier
func (user *User) GetConfirmVerifier() string {
	return user.ConfirmVerifier
}

// PutConfirmVerifier stores the user's confirmation verifier
func (user *User) PutConfirmVerifier(verifier string) {
	user.ConfirmVerifier = verifier
}

// GetAttemptCount returns the number of login attempts prior to the
// user's account being locked.
func (user *User) GetAttemptCount() (attempts int) {
	return user.AttemptCount
}

// PutAttemptCount stores the number of login attempts prior to
// the account being locked.
func (user *User) PutAttemptCount(attempts int) {
	user.AttemptCount = attempts
}

// GetLastAttempt returns the last unsuccessful attempt time
func (user *User) GetLastAttempt() (last time.Time) {
	return user.LastAttempt
}

// PutLastAttempt stores the last unsuccessful attempt time
func (user *User) PutLastAttempt(last time.Time) {
	user.LastAttempt = last
}

// GetLocked returns the user's account lock status
func (user *User) GetLocked() (locked time.Time) {
	return user.Locked
}

// PutLocked stores the user's account lock status
func (user *User) PutLocked(locked time.Time) {
	user.Locked = locked
}

// GetRecoverSelector returns the recovery selector (URL)
func (user *User) GetRecoverSelector() (selector string) {
	return user.RecoverSelector
}

// PutRecoverSelector stores the recovery selector (URL)
func (user *User) PutRecoverSelector(selector string) {
	user.RecoverSelector = selector
}

// GetRecoverVerifier returns the recovery verifier (URL)
func (user *User) GetRecoverVerifier() (verifier string) {
	return user.RecoverVerifier
}

// PutRecoverVerifier stores the recovery verifier (URL)
func (user *User) PutRecoverVerifier(verifier string) {
	user.RecoverVerifier = verifier
}

// GetRecoverExpiry returns the recovery process' expiration time
func (user *User) GetRecoverExpiry() (expiry time.Time) {
	return user.RecoverTokenExpiry
}

// PutRecoverExpiry stores the recovery process' expiration time
func (user *User) PutRecoverExpiry(expiry time.Time) {
	user.RecoverTokenExpiry = expiry
}

// GetTOTPSecretKey returns the time-based one time password (TOTP) secret key
func (user *User) GetTOTPSecretKey() string {
	return user.TOTPSecretKey
}

// PutTOTPSecretKey stores the time-based one time password (TOTP) secret key
func (user *User) PutTOTPSecretKey(totpSecret string) {
	user.TOTPSecretKey = totpSecret
}

// GetRecoveryCodes retrieves a CSV string of bcrypt'd recovery codes
func (user *User) GetRecoveryCodes() string {
	return user.RecoveryCodes
}

// PutRecoveryCodes uses a single string to store many bcrypt'd recovery codes
func (user *User) PutRecoveryCodes(codes string) {
	user.RecoveryCodes = codes
}

// GetSMSPhoneNumber returns the user's phone number to which a text message
// with a 2FA code will be sent
func (user *User) GetSMSPhoneNumber() string {
	return user.SMSPhoneNumber
}

// PutSMSPhoneNumber returns the user's phone number to which a text message
// will be sent
func (user *User) PutSMSPhoneNumber(phnumber string) {
	user.SMSPhoneNumber = phnumber
}
