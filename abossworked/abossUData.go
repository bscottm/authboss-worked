package abossworked

/* "scooter me fecit"

Copyright 2022 B. Scott Michel

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program. If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	// Authboss
	"github.com/google/uuid"
	"github.com/volatiletech/authboss/v3"

	// TBD: "github.com/volatiletech/authboss/v3/otp/twofactor/sms2fa"
	// TBD: "github.com/volatiletech/authboss/v3/otp/twofactor/totp2fa"

	// GORM
	// If you prefer the CGO driver to SQLite, uncomment:
	// "gorm.io/driver/sqlite"
	//
	// And comment out the line below.
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	workedUserdb = "worked_udata.sqlite3"
)

// AuthStorer holds the SQLite database state
type AuthStorer struct {
	// Logging instance
	log *log.Logger
	// GORM's connection to the SQLite database...
	UserDB *gorm.DB
}

// WorkedUser is the glue structure that connects user state to Authboss.
// It embeds the AuthStorer SQL state with the UserData user data so that we
// don't have to store everything in UserData and can separate out Authboss
// functions, such as confirmation and account locking.
type WorkedUser struct {
	*AuthStorer
	UserData

	// Authboss "arbitrary" data map.
	arbitraryData map[string]string
}

// This pattern is useful in real code to ensure that
// we've got the right interfaces implemented.
var (
	assertUser   = &WorkedUser{}
	assertStorer = &AuthStorer{}

	_ authboss.User            = assertUser
	_ authboss.AuthableUser    = assertUser
	_ authboss.ConfirmableUser = assertUser
	_ authboss.LockableUser    = assertUser
	_ authboss.RecoverableUser = assertUser
	_ authboss.ArbitraryUser   = assertUser

	// TBD: _ totp2fa.User = assertUser
	// TBD: _ sms2fa.User  = assertUser

	_ authboss.ServerStorer            = assertStorer
	_ authboss.CreatingServerStorer    = assertStorer
	_ authboss.ConfirmingServerStorer  = assertStorer
	_ authboss.RecoveringServerStorer  = assertStorer
	_ authboss.RememberingServerStorer = assertStorer
)

// OpenUserDB opens the user database and creates the database structure if it
// doesn't already exist.
func OpenUserDB(workedRoot string) (storer *AuthStorer, err error) {
	storer = &AuthStorer{}

	storer.log = log.New(os.Stdout, "[USERDB] ", log.LstdFlags)
	storeLogger := logger.New(
		storer.log,
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	workedUserDBPath := strings.Join([]string{workedRoot, workedUserdb}, string(os.PathSeparator))
	storer.log.Printf("userdb path %s", workedUserDBPath)

	storer.UserDB, err = gorm.Open(sqlite.Open(workedUserDBPath), &gorm.Config{
		Logger: storeLogger,
	})

	if err != nil {
		return nil, err
	}

	// Create or update the database's structure:
	userDBTables := []interface{}{
		&UserData{},
		&Confirmations{},
		&LockedAccount{},
		&RecoveryRequests{},
		&RememberMeTokens{},
	}

	return storer, storer.UserDB.AutoMigrate(userDBTables...)
}

// Close and cleanup for SQLStorer.
func (storer *AuthStorer) Close() {
	// Really. Close the database connection.
	sqlDB, err := storer.UserDB.DB()
	if err == nil {
		storer.log.Println("Closing userdb.")
		sqlDB.Close()
	}

	storer.UserDB = nil
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// ServerStorer interface:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// Load will look up the user based on the passed the PrimaryID. Under
// normal circumstances this comes from GetPID() of the user.
//
// OAuth2 logins are special-cased to return an OAuth2 pid (combination of
// provider:oauth2uid), and therefore key be special cased in a Load()
// implementation to handle that form, use ParseOAuth2PID to see
// if key is an OAuth2PID or not.
func (storer AuthStorer) Load(ctx context.Context, key string) (authboss.User, error) {
	var tx *gorm.DB
	workedUser := &WorkedUser{
		AuthStorer: &storer,
		UserData:   UserData{},
	}

	/* OAuth2: TBD
	if provider, oauthUID, err := authboss.ParseOAuth2PID(key); err == nil {
		tx = storer.UserDB.Model(&UserData{}).
			Where(UserData{
				OAuth2UID:      sql.NullString{String: oauthUID, Valid: true},
				OAuth2Provider: provider,
			}).First(&workedUser)
	} else */

	// Lookup by Primary User Identifier (email)
	tx = storer.UserDB.Model(&UserData{}).Where(UserData{Email: key}).First(&workedUser.UserData)

	storer.log.Printf("Load(ctx, %v) -> %v", key, workedUser)
	if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, authboss.ErrUserNotFound
	} else if tx.Error != nil {
		return nil, tx.Error
	}

	return workedUser, nil
}

// Save persists the user in the database. This should never
// create a user and instead return ErrUserNotFound if the user
// does not exist.
func (storer AuthStorer) Save(ctx context.Context, abUser authboss.User) error {
	user, valid := abUser.(*WorkedUser)
	if !valid {
		storer.log.Printf("Save(): Expected a User struct in authboss.User annotation.")
		return errors.New("expected a User struct in authboss.User annotation in Save()")
	}

	dbUser := storer.UserDB.Model(&UserData{}).Where(UserData{Email: user.GetPID()}).First(&UserData{})
	if dbUser.RowsAffected == 0 {
		return authboss.ErrUserNotFound
	}

	result := storer.UserDB.Save(user.UserData)
	storer.log.Printf("Save(): rows affected %v", result.RowsAffected)

	return result.Error
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// CreatingServerStorer interface:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// New creates a blank user. It is not yet persisted in the database.
func (storer AuthStorer) New(ctx context.Context) authboss.User {
	return &WorkedUser{
		AuthStorer: &storer,
		UserData:   UserData{},
	}
}

// Create the user in the SQLite udata table, returning authboss.ErrUserFound if
// the user already exists.
func (storer AuthStorer) Create(ctx context.Context, abUser authboss.User) error {
	// Yes, a little "tongue in cheek" humor on the type annotation
	user, valid := abUser.(*WorkedUser)

	if !valid {
		storer.log.Printf("Expected a User struct in authboss.User annotation.")
		return errors.New("expected a User struct in authboss.User annotation")
	}

	if len(user.UserData.GUID) > 0 {
		rows := storer.UserDB.Model(&UserData{}).Where(UserData{GUID: user.UserData.GUID}).First(&UserData{})
		if rows.RowsAffected > 0 {
			// Have at least one row...
			storer.log.Printf("Duplicate user (2): GUID %s e-mail %s", user.UserData.GUID, user.UserData.Email)
			return authboss.ErrUserFound
		}
		if rows.Error != nil && !errors.Is(rows.Error, gorm.ErrRecordNotFound) {
			return rows.Error
		}
	} else {
		// Generate a new UUID for the user, dashes and all. The GUID could have been stored as a
		// byte array or as a BASE64 string. Using the string representation with dashes is a
		// reasonable and readable compromise representation.
		user.UserData.GUID = uuid.New().String()
	}

	// Also check email uniqueness:
	emailExists := storer.UserDB.Model(&UserData{}).Where(UserData{Email: user.GetPID()}).First(&UserData{})
	if emailExists.RowsAffected > 0 {
		storer.log.Printf("Duplicate user (3): GUID %s e-mail %s", user.UserData.GUID, user.UserData.Email)
		return authboss.ErrUserFound
	}
	if emailExists.Error != nil && !errors.Is(emailExists.Error, gorm.ErrRecordNotFound) {
		// Different, unspecified error
		return emailExists.Error
	}

	// SQL INSERT:
	result := storer.UserDB.Model(&UserData{}).Create(&user.UserData)

	if result.Error == nil {
		storer.log.Printf("Successfully added user: GUID %s e-mail %s", user.UserData.GUID, user.UserData.Email)
	} else {
		storer.log.Print(fmt.Errorf("error adding user GUID %s/e-mail %s: %w", user.UserData.GUID, user.UserData.Email, result.Error))
	}

	return result.Error
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// ConfirmingServerStorer implementation
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// LoadByConfirmSelector loads the user via their confirmation selector.
func (storer AuthStorer) LoadByConfirmSelector(ctx context.Context, selector string) (authboss.ConfirmableUser, error) {
	var confirm Confirmations

	// GORM association join:
	result := storer.UserDB.Model(&Confirmations{}).
		Where(Confirmations{Selector: makeSQLNullString(selector)}).
		Joins("User").
		First(&confirm)

	storer.log.Printf("LoadByConfirmSelector result: %v", confirm)

	if result.Error != nil {
		return &WorkedUser{}, result.Error
	} else if result.RowsAffected == 0 {
		return &WorkedUser{}, authboss.ErrUserNotFound
	} else if len(confirm.User.GUID) == 0 {
		storer.log.Fatalf("LoadByConfirmSelector Association to User did not find GUID.")
		// NOTREACHED
		// Should never happen. Truly.
		return &WorkedUser{}, authboss.ErrUserNotFound
	}

	return &WorkedUser{
		AuthStorer:    &storer,
		UserData:      confirm.User,
		arbitraryData: map[string]string{},
	}, nil
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// RecoveringServerStorer implementation
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// LoadByRecoverSelector loads the user using the recovery selector string.
func (storer AuthStorer) LoadByRecoverSelector(ctx context.Context, selector string) (authboss.RecoverableUser, error) {
	var recovery RecoveryRequests

	// GORM association join:
	result := storer.UserDB.Model(&RecoveryRequests{}).
		Where(RecoveryRequests{Selector: makeSQLNullString(selector)}).
		Joins("User").
		First(&recovery)

	storer.log.Printf("LoadByRecoverySelector result: %v", recovery)

	if result.Error != nil {
		return &WorkedUser{}, result.Error
	} else if result.RowsAffected == 0 {
		return &WorkedUser{}, authboss.ErrUserNotFound
	} else if len(recovery.User.GUID) == 0 {
		storer.log.Fatalf("LoadByConfirmSelector Association to User did not find GUID.")
		// NOTREACHED
		// Should never happen. Truly.
		return &WorkedUser{}, authboss.ErrUserNotFound
	}

	return &WorkedUser{
		AuthStorer:    &storer,
		UserData:      recovery.User,
		arbitraryData: map[string]string{},
	}, nil
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// RememberingServerStorer implementation
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// AddRememberToken associates a "remember me" token with the user
func (storer AuthStorer) AddRememberToken(ctx context.Context, pid, token string) error {
	var userGUID string

	tx := storer.UserDB.Model(&UserData{}).Select("guid").Where(UserData{Email: pid}).First(&userGUID)
	if tx.Error != nil {
		return tx.Error
	}

	guidTX := storer.UserDB.Model(&RememberMeTokens{}).Create(&RememberMeTokens{GUID: userGUID, Token: token})

	storer.log.Printf("AddRememberToken: %v rows affected.", guidTX.RowsAffected)
	return guidTX.Error
}

// DelRememberTokens removes all "remember me" tokens previously associated with the user
func (storer AuthStorer) DelRememberTokens(ctx context.Context, pid string) error {
	var userGUID string

	tx := storer.UserDB.Model(&UserData{}).Select("guid").Where(UserData{Email: pid}).First(&userGUID)
	if tx.Error != nil {
		return tx.Error
	}

	// GUID is not the primary key, so a "WHERE" clause is required to delete:
	guidToDelete := RememberMeTokens{GUID: userGUID}
	guidTX := storer.UserDB.Model(&guidToDelete).Where(&guidToDelete).Delete(&guidToDelete)

	storer.log.Printf("DelRememberTokens: %v rows deleted.", guidTX.RowsAffected)
	return guidTX.Error
}

// UseRememberToken finds the pid-token pair and deletes it.
// If the token could not be found return ErrTokenNotFound
func (storer AuthStorer) UseRememberToken(ctx context.Context, pid, token string) error {
	var userGUID string

	tx := storer.UserDB.Model(&UserData{}).Select("guid").Where(UserData{Email: pid}).First(&userGUID)
	if tx.Error != nil {
		return tx.Error
	}

	guidTX := storer.UserDB.Model(&RememberMeTokens{}).Delete(&RememberMeTokens{GUID: userGUID, Token: token})

	storer.log.Printf("UseRememberToken: %v rows affected (GUID: %v, token %v).", guidTX.RowsAffected, userGUID, token)
	return guidTX.Error
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// Getter/Setter Authboss interfaces between WorkedUser and Authboss functionality:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// GetPID returns the user's primary identifier, which is user's email address
func (user *WorkedUser) GetPID() string {
	return user.Email
}

// PutPID stores the user's identifier in the User structure, interface function for authboss.User
func (user *WorkedUser) PutPID(pid string) {
	user.Email = pid
}

// GetPassword returns the bcrypt-ed user password
func (user *WorkedUser) GetPassword() string {
	return user.UIDData
}

// PutPassword stores the bcrypt-ed user password
func (user *WorkedUser) PutPassword(pass string) {
	user.UIDData = pass
}

// GetEmail returns the user's e-mail address, which also happens to be the PID
func (user *WorkedUser) GetEmail() (email string) {
	return user.Email
}

// PutEmail stores the user's e-mail address, which also happens to be the PID
func (user *WorkedUser) PutEmail(email string) {
	user.Email = email
}

/* Not used at the moment. An example of how to use GORM Associations via the Joins() method to grab the
   entire UserData structure associated with a Confirmations structure type.

   func (user *WorkedUser) getConfirmationByGUID() (confirm Confirmations, err error) {
	// This might seem to be overkill -- joining back to the UserData user via the user's GUID.
	// It does ensure that the user really does exist.
	result := user.AuthStorer.UserDB.Model(&Confirmations{}).
		Where(Confirmations{GUID: user.GUID}).
		Joins("User").
		First(&confirm)

	if result.Error != nil {
		// Error or inner join fail.
		return Confirmations{}, result.Error
	} else if len(confirm.User.GUID) == 0 {
		return Confirmations{}, errors.New("inner join to UserData failed")
	}

	return confirm, nil
}
*/

// GetConfirmed returns the user's confirmation status
func (user *WorkedUser) GetConfirmed() (confirmed bool) {
	/*
		confirm, err := user.getConfirmationByGUID()
		if err != nil {
			return false
		}

		return confirm.Confirmed
	*/

	// Use a GORM subquery here to reduce some pressure on the DB, while also
	// verifying that the user actually exists in the udata table.

	subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
	result := user.AuthStorer.UserDB.Model(&Confirmations{}).
		Select("Confirmed").
		Where("GUID IN (?)", subq).
		First(&confirmed)

	if result.Error == nil {
		return confirmed
	}

	return false
}

// PutConfirmed stores the user's confirmation status
func (user *WorkedUser) PutConfirmed(confirmed bool) {
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"confirmed"}),
	}).Create(&Confirmations{
		GUID:      user.GUID,
		Confirmed: confirmed,
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutConfirmed failed: %v", tx.Error)
	}
}

// GetConfirmSelector returns the user's confirmation selector (URL)
func (user *WorkedUser) GetConfirmSelector() string {
	/*
		confirm, err := user.getConfirmationByGUID()
		if err != nil {
			return ""
		}

		return confirm.Selector.String
	*/

	// Use a GORM subquery here to reduce some pressure on the DB, while also
	// verifying that the user actually exists in the udata table.

	var sqlSelector sql.NullString

	subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
	result := user.AuthStorer.UserDB.Model(&Confirmations{}).
		Select("Selector").
		Where("GUID IN (?)", subq).
		First(&sqlSelector)

	if result.Error == nil && sqlSelector.Valid {
		return sqlSelector.String
	}

	return ""
}

// PutConfirmSelector stores the user's confirmation selector
func (user *WorkedUser) PutConfirmSelector(selector string) {
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"selector"}),
	}).Create(&Confirmations{
		GUID:     user.GUID,
		Selector: makeSQLNullString(selector),
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutConfirmSelector failed: %v", tx.Error)
	}
}

// GetConfirmVerifier returns the user's confirmation verifier
func (user *WorkedUser) GetConfirmVerifier() string {
	/*
		confirm, err := user.getConfirmationByGUID()
		if err != nil {
			return ""
		}

		return confirm.Verifier.String
	*/

	// Use a GORM subquery here to reduce some pressure on the DB, while also
	// verifying that the user actually exists in the udata table.

	var sqlVerifier sql.NullString

	subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
	result := user.AuthStorer.UserDB.Model(&Confirmations{}).
		Select("Verifier").
		Where("GUID IN (?)", subq).
		First(&sqlVerifier)

	if result.Error == nil && sqlVerifier.Valid {
		return sqlVerifier.String
	}

	return ""
}

// PutConfirmVerifier stores the user's confirmation verifier
func (user *WorkedUser) PutConfirmVerifier(verifier string) {
	// UPSERT to update the verifier -- if the GUID already exists, and it likely does,
	// then only update the verifier.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"verifier"}),
	}).Create(&Confirmations{
		GUID:     user.GUID,
		Verifier: makeSQLNullString(verifier),
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutConfirmVerifier failed: %v", tx.Error)
	}
}

/* Not used at the moment. An example of how to use GORM Associations via the Joins() method to grab the
   entire UserData structure associated with a LockedAccount.

   func (user *WorkedUser) getLockedByGUID() (lockout LockedAccount, err error) {
	// This might seem to be overkill -- joining back to the UserData user via the user's GUID.
	// It does ensure that the user really does exist.
	result := user.AuthStorer.UserDB.Model(&LockedAccount{}).
		Where(LockedAccount{GUID: user.GUID}).
		Joins("User").
		First(&lockout)

	if result.Error != nil {
		// Error or inner join fail.
		return LockedAccount{}, result.Error
	} else if len(lockout.User.GUID) == 0 {
		return LockedAccount{}, errors.New("inner join to UserData failed")
	}

	return lockout, nil
}
*/

// GetAttemptCount returns the number of login attempts prior to the
// user's account being locked.
func (user *WorkedUser) GetAttemptCount() (attempts int) {
	/*
		lockout, err := user.getLockedByGUID()
		if err != nil {
			return 99
		}

		return lockout.AttemptCount
	*/

	// Use a GORM subquery here to reduce some pressure on the DB, while also
	// verifying that the user actually exists in the udata table.

	subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
	result := user.AuthStorer.UserDB.Model(&LockedAccount{}).
		Select("AttemptCount").
		Where("GUID IN (?)", subq).
		First(&attempts)

	if result.Error != nil {
		attempts = 9999
	}

	return attempts
}

// PutAttemptCount stores the number of login attempts prior to
// the account being locked.
func (user *WorkedUser) PutAttemptCount(attempts int) {
	// UPSERT to update the attempt count -- if the GUID already exists, and it likely does,
	// then only update the attempt count.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"attempt_count"}),
	}).Create(&LockedAccount{
		GUID:         user.GUID,
		AttemptCount: attempts,
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutAttemptCount failed: %v", tx.Error)
	}
}

// GetLastAttempt returns the last unsuccessful attempt time
func (user *WorkedUser) GetLastAttempt() (last time.Time) {
	/*
		lockout, err := user.getLockedByGUID()
		if err != nil {
			return time.Time{}
		}

		return lockout.LastAttempt
	*/

	// See note in GetAttemptCount.

	subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
	result := user.AuthStorer.UserDB.Model(&LockedAccount{}).
		Select("LastAttempt").
		Where("GUID IN (?)", subq).
		First(&last)

	if result.Error != nil {
		last = time.Time{}
	}

	return last
}

// PutLastAttempt stores the last unsuccessful attempt time
func (user *WorkedUser) PutLastAttempt(last time.Time) {
	// UPSERT to update the last attempt time -- if the GUID already exists, and it likely does,
	// then only update the last attempt time.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_attempt"}),
	}).Create(&LockedAccount{
		GUID:        user.GUID,
		LastAttempt: last,
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutLastAttempt failed: %v", tx.Error)
	}
}

// GetLocked returns the user's account lock status
func (user *WorkedUser) GetLocked() (locked time.Time) {
	/*
		lockout, err := user.getLockedByGUID()
		if err != nil {
			return time.Time{}
		}

		return lockout.Locked
	*/

	// See note in GetAttemptCount.

	subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
	result := user.AuthStorer.UserDB.Model(&LockedAccount{}).
		Select("Locked").
		Where("GUID IN (?)", subq).
		First(&locked)

	if result.Error != nil {
		locked = time.Time{}
	}

	return locked
}

// PutLocked stores the user's account lock status
func (user *WorkedUser) PutLocked(locked time.Time) {
	// UPSERT to update the account lock status -- if the GUID already exists, and it likely does,
	// then only update the status.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"locked"}),
	}).Create(&LockedAccount{
		GUID:   user.GUID,
		Locked: locked,
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutLocked failed: %v", tx.Error)
	}
}

// getRecoveryByGUID gets the user's recovery data by the user's GUID.
func (user *WorkedUser) getRecoveryByGUID() (recovery RecoveryRequests, err error) {
	// This might seem to be overkill -- joining back to the UserData user via the user's GUID.
	// It does ensure that the user really does exist.
	result := user.AuthStorer.UserDB.Model(&RecoveryRequests{}).
		Where(RecoveryRequests{GUID: user.GUID}).
		Joins("User").
		First(&recovery)

	if result.Error != nil {
		// Error or inner join fail.
		return RecoveryRequests{}, result.Error
	} else if len(recovery.User.GUID) == 0 {
		return RecoveryRequests{}, errors.New("inner join to UserData failed")
	}

	return recovery, nil
}

// GetRecoverSelector returns the recovery selector (URL)
func (user *WorkedUser) GetRecoverSelector() (selector string) {
	recovery, err := user.getRecoveryByGUID()
	if err != nil {
		return ""
	}

	return recovery.Selector.String
}

// PutRecoverSelector stores the recovery selector (URL)
func (user *WorkedUser) PutRecoverSelector(selector string) {
	// UPSERT to update the account lock status -- if the GUID already exists, and it likely does,
	// then only update the status.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"selector"}),
	}).Create(&RecoveryRequests{
		GUID:     user.GUID,
		Selector: makeSQLNullString(selector),
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutRecoverySelector failed: %v", tx.Error)
	}
}

// GetRecoverVerifier returns the recovery verifier (URL)
func (user *WorkedUser) GetRecoverVerifier() (verifier string) {
	recovery, err := user.getRecoveryByGUID()
	if err != nil {
		return ""
	}

	return recovery.Verifier.String
}

// PutRecoverVerifier stores the recovery verifier (URL)
func (user *WorkedUser) PutRecoverVerifier(verifier string) {
	// UPSERT to update the account lock status -- if the GUID already exists, and it likely does,
	// then only update the status.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"verifier"}),
	}).Create(&RecoveryRequests{
		GUID:     user.GUID,
		Verifier: makeSQLNullString(verifier),
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutRecoveryVerifier failed: %v", tx.Error)
	}

}

// GetRecoverExpiry returns the recovery process' expiration time
func (user *WorkedUser) GetRecoverExpiry() (expiry time.Time) {
	recovery, err := user.getRecoveryByGUID()
	if err != nil {
		return time.Time{}
	}

	return recovery.TokenExpiry
}

// PutRecoverExpiry stores the recovery process' expiration time
func (user *WorkedUser) PutRecoverExpiry(expiry time.Time) {
	// UPSERT to update the account lock status -- if the GUID already exists, and it likely does,
	// then only update the status.
	tx := user.AuthStorer.UserDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "guid"}},
		DoUpdates: clause.AssignmentColumns([]string{"token_expiry"}),
	}).Create(&RecoveryRequests{
		GUID:        user.GUID,
		TokenExpiry: expiry,
	})

	if tx.Error != nil {
		user.AuthStorer.log.Printf("PutRecoveryExpiry failed: %v", tx.Error)
	}

}

/* 2FA: TBD
// GetTOTPSecretKey returns the time-based one time password (TOTP) secret key
func (user *WorkedUser) GetTOTPSecretKey() string {
	return user.TOTPSecretKey
}

// PutTOTPSecretKey stores the time-based one time password (TOTP) secret key
func (user *WorkedUser) PutTOTPSecretKey(totpSecret string) {
	user.TOTPSecretKey = totpSecret
}

// GetRecoveryCodes retrieves a CSV string of bcrypt'd recovery codes
func (user *WorkedUser) GetRecoveryCodes() string {
	return user.RecoveryCodes
}

// PutRecoveryCodes uses a single string to store many bcrypt'd recovery codes
func (user *WorkedUser) PutRecoveryCodes(codes string) {
	user.RecoveryCodes = codes
}

// GetSMSPhoneNumber returns the user's phone number to which a text message
// with a 2FA code will be sent
func (user *WorkedUser) GetSMSPhoneNumber() string {
	return user.SMSPhoneNumber
}

// PutSMSPhoneNumber returns the user's phone number to which a text message
// will be sent
func (user *WorkedUser) PutSMSPhoneNumber(phnumber string) {
	user.SMSPhoneNumber = phnumber
}
*/

// GetArbitrary returns the authboss "arbitrary" form data that should be preserved across
// form invocations.
func (user *WorkedUser) GetArbitrary() (arbitrary map[string]string) {
	return user.arbitraryData
}

// PutArbitrary stores the authboss "arbitrary" form data that should be preserved across
// form invocations (e.g.., validation failed, but you'd like to keep the user's e-mail.)
func (user *WorkedUser) PutArbitrary(arbitrary map[string]string) {
	user.arbitraryData = make(map[string]string, len(arbitrary))
	for _, k := range arbitrary {
		user.arbitraryData[k] = arbitrary[k]
	}
}

// makeSQLNullString converts Go strings to SQL NULL objects. If the string's
// length == 0, then it's considered a SQL NULL value. (You would think a
// convenience function like this would already exist...)
//
// Note: The majority of the time, len(string) > 0, so it's the more likely
// branch path.
func makeSQLNullString(str string) sql.NullString {
	if len(str) > 0 {
		return sql.NullString{String: str, Valid: true}
	}

	return sql.NullString{Valid: false}
}
