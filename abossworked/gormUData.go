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

/* These are all of the structure types that are stored in the GORM database database. */

import (
	"database/sql"
	"time"
)

// UserData is the underlying database object structure
type UserData struct {
	// User's GUID, the primary relation to other tables and (potentially) other
	// databases. References the user using a unique value rather than by their e-mail
	// or Authboss PID when joining across tables or databases.
	GUID string `gorm:"primaryKey;not null;type:char(36)"`

	// E-mail in this example code is the user's primary unique identifier ("PID" in
	// the Authboss documentation and code.) Could also be a user name or ... (and maybe
	// consider renaming this member to "PID").
	Email string `gorm:"uniqueIndex;not null;type:varchar(256)"`
	// bCrypt-ed password
	UIDData string `gorm:"column:uid_data;not null;type:varchar(64)"`

	// OAuth2: TBD
	/*
		OAuth2UID          sql.NullString `gorm:"uniqueIndex"`
		OAuth2Provider     string
		OAuth2AccessToken  string
		OAuth2RefreshToken string
		OAuth2Expiry       time.Time
	*/

	// 2fa: TBD
	/*
		TOTPSecretKey      string
		SMSPhoneNumber     string
		SMSSeedPhoneNumber string
		RecoveryCodes      string
	*/

	// Remember is in another table

	// GORM's Model members:
	CreatedAt time.Time
	UpdatedAt time.Time
	// If you want to use GORM's "soft delete", uncomment
	// DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName for the UserData structure is "udata", not the GORM default "u_data"
func (UserData) TableName() string {
	return "udata"
}

// Confirmations is the underlying database table object for confirmation
// data. It has an inverted relationship with UserData: while it might have
// made more sense to embed Confirmations in the UserData structure, inverting
// the relationship ensures that a confirmation has a corresponding user.
type Confirmations struct {
	GUID      string         `gorm:"primaryKey;not null;type:char(36)"`
	Selector  sql.NullString `gorm:"uniqueIndex"`
	Verifier  sql.NullString `gorm:"uniqueIndex"`
	Confirmed bool

	// 1-to-1 association with UserData via GUID join
	User UserData `gorm:"foreignKey:GUID"`

	// GORM's Model members:
	CreatedAt time.Time
	UpdatedAt time.Time
	// If you want to use GORM's "soft delete", uncomment
	// DeletedAt gorm.DeletedAt `gorm:"index"`
}

// LockedAccount is the underlying database table object for locking user
// accounts when there have been too many unsuccessful authentication attempts.
type LockedAccount struct {
	GUID         string `gorm:"primaryKey;not null;type:char(36)"`
	AttemptCount int
	LastAttempt  time.Time
	Locked       time.Time

	// 1-to-1 association with UserData via GUID join
	User UserData `gorm:"foreignKey:GUID"`

	// GORM's Model members:
	CreatedAt time.Time
	UpdatedAt time.Time
	// If you want to use GORM's "soft delete", uncomment
	// DeletedAt gorm.DeletedAt `gorm:"index"`
}

// RecoveryRequests is the underlying database table object for tracking account
// recovery requests.
type RecoveryRequests struct {
	GUID        string         `gorm:"primaryKey;not null;type:char(36)"`
	Selector    sql.NullString `gorm:"uniqueIndex"`
	Verifier    sql.NullString `gorm:"uniqueIndex"`
	TokenExpiry time.Time

	// 1-to-1 association with UserData via GUID join
	User UserData `gorm:"foreignKey:GUID"`

	// GORM's Model members:
	CreatedAt time.Time
	UpdatedAt time.Time
	// If you want to use GORM's "soft delete", uncomment
	// DeletedAt gorm.DeletedAt `gorm:"index"`
}

// RememberMeTokens is the underlying database table object for Primary IDentifier
// and remember-me tokens. This is intentionally disconnected (no direct foreign key
// relationship, no association) from the UserData table.
type RememberMeTokens struct {
	// User's GUID: This will not be unique, since the user can use multiple browsers.
	GUID string `gorm:"not null;index;type:char(36)"`
	// Remember-me token. There can be multiple tokens associated with the
	// user, each of which are distinct.
	Token string `gorm:"primaryKey;not null"`

	// Really needs an expiration to reap the token...
}

// TableName returns the "remember" table name for RememberMeTokens.
func (RememberMeTokens) TableName() string {
	return "remember"
}
