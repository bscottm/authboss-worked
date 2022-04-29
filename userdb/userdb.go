// Package userdb is a SQLite3-based Authboss "Storer" and implements the various "user"
// interfaces (authboss.User, authboss.AuthUser, authboss.ConfirmableUser, ...)
package userdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	// Authboss
	"github.com/volatiletech/authboss/v3"
	"github.com/volatiletech/authboss/v3/otp/twofactor/sms2fa"
	"github.com/volatiletech/authboss/v3/otp/twofactor/totp2fa"

	// GUID generator
	"github.com/google/uuid"

	// Get all of the import side effects without having a named module.
	_ "modernc.org/sqlite"
)

const (
	explanUserdb = "explan_udata.sqlite3"

	SQLITE_SIGNATURE_MISMATCH = 0
	SQLITE_NO_SCHEMA          = 1
	SQLITE_OPEN_ERR           = 2
)

// SQLStorer holds the SQLite database state
type SQLStorer struct {
	// Logging instance
	log *log.Logger
	// Open connection to the SQLite database...
	userdb *sql.DB
}

// This pattern is useful in real code to ensure that
// we've got the right interfaces implemented.
var (
	assertUser   = &User{}
	assertStorer = &SQLStorer{}

	_ authboss.User            = assertUser
	_ authboss.AuthableUser    = assertUser
	_ authboss.ConfirmableUser = assertUser
	_ authboss.LockableUser    = assertUser
	_ authboss.RecoverableUser = assertUser
	// _ authboss.ArbitraryUser   = assertUser

	_ totp2fa.User = assertUser
	_ sms2fa.User  = assertUser

	_ authboss.CreatingServerStorer = assertStorer
	// _ authboss.ConfirmingServerStorer  = assertStorer
	// _ authboss.RecoveringServerStorer  = assertStorer
	// _ authboss.RememberingServerStorer = assertStorer

	errStrings = map[int]string{
		SQLITE_SIGNATURE_MISMATCH: "SQLite schema did not match.",
		SQLITE_NO_SCHEMA:          "Missing SQLite schema.",
		SQLITE_OPEN_ERR:           "Could not open " + explanUserdb + " database",
	}
)

// userdbError encodes errors encountered in this module.
type userdbError struct {
	code       int
	wrappedErr error
}

func (uerr *userdbError) Error() string {
	errstr, valid := errStrings[uerr.code]
	if valid {
		if uerr.wrappedErr == nil {
			return errstr
		}

		return fmt.Sprintf("%s: %s", errstr, fmt.Errorf("%w", uerr.wrappedErr))
	}

	return "Perplexion in userdbError"
}

// OpenUserDB Open the user database and create the structure if it doesn't already
// exist.
func OpenUserDB(sqlSchemaSig string) (*SQLStorer, error) {
	var storer *SQLStorer = &SQLStorer{}
	var err error

	storer.log = log.New(os.Stdout, "userdb: ", log.LstdFlags)
	storer.userdb, err = sql.Open("sqlite", explanUserdb)
	if err == nil {
		// Generate the SH256 signature on the explanUserdb's schema, straight from
		// the SQLite schema:
		var rows *sql.Rows

		rows, err = storer.userdb.Query("select sql from sqlite_schema;")
		if err == nil {
			defer rows.Close()

			var dbschemas []string

			for i := 0; rows.Next(); i++ {
				var row string

				rows.Scan(&row)
				dbschemas = append(dbschemas, row)
			}

			dbschema := strings.Join(dbschemas, "\n") + "\n"

			/*
				// Should make this a debug thing...
				fmt.Print(dbschema)

				f, err := os.Create("zzz.sql")
				if err == nil {
					defer f.Close()
					fmt.Fprint(f, dbschema)
				}
			*/

			var sig = sha256.Sum256([]byte(dbschema))
			var sigString = fmt.Sprintf("%x", sig[:])

			log.Printf("SHA256 on SQLite schema: %s", sigString)
			if sigString != sqlSchemaSig {
				log.Printf("Expected:                %s", sqlSchemaSig)
				err = &userdbError{SQLITE_SIGNATURE_MISMATCH, nil}
			} else {
				log.Printf("SHA256 on SQLite schema -- good signature.")
			}
		} else {
			log.Printf("no sqlite_schema: %v", err)
			err = &userdbError{SQLITE_NO_SCHEMA, nil}
		}
	} else {
		log.Printf("could not open %s: %v", explanUserdb, err)
		err = &userdbError{SQLITE_OPEN_ERR, err}
	}

	return storer, err
}

// Close and cleanup for SQLStorer.
func (storer *SQLStorer) Close() {
	storer.userdb.Close()
	storer.userdb = nil
}

// ServerStorer interface:

// Load will look up the user based on the passed the PrimaryID. Under
// normal circumstances this comes from GetPID() of the user.
//
// OAuth2 logins are special-cased to return an OAuth2 pid (combination of
// provider:oauth2uid), and therefore key be special cased in a Load()
// implementation to handle that form, use ParseOAuth2PID to see
// if key is an OAuth2PID or not.
func (db SQLStorer) Load(ctx context.Context, key string) (authboss.User, error) {

	user := &User{}

	if provider, outhUID, err := authboss.ParseOAuth2PID(key); err != nil {
		user.OAuth2Provider = provider
		user.OAuth2UID = outhUID
	} else if strings.Contains(key, "@") {
	} else {
		_, _ = db.userdb.Query("SELECT * from udata where uid=?", key)
	}

	return user, nil
}

// Save persists the user in the database, this should never
// create a user and instead return ErrUserNotFound if the user
// does not exist.
func (db SQLStorer) Save(ctx context.Context, user authboss.User) error {
	return nil
}

// CreatingServerStorer interface:

// New creates a blank user, it is not yet persisted in the database
// but is just for storing data
func (db SQLStorer) New(ctx context.Context) authboss.User {
	return &User{}
}

// Create the user in the SQLite udata table, returning authboss.ErrUserFound if
// the user already exists.
func (db SQLStorer) Create(ctx context.Context, abUser authboss.User) error {
	user := abUser.(*User)

	if len(user.GUID) > 0 {
		rows, err := db.userdb.QueryContext(ctx, "SELECT uid FROM udata WHERE uid=?", user.GUID)
		if err != nil {
			return err
		}

		defer rows.Close()
		if rows.Next() {
			// Have at least one row...
			db.log.Printf("Duplicate user: GUID %s e-mail %s", user.GUID, user.Email)
			return authboss.ErrUserFound
		}
	} else {
		// Generate a new UUID for the user, dashes and all. The GUID could have been stored as a
		// byte array or as a BASE64 string. Using the string representation with dashes is a
		// reasonable and readable compromise representation.
		user.GUID = uuid.New().String()
	}

	_, err := db.userdb.ExecContext(ctx, "INSERT INTO udata VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		user.GUID, "**invalid**", user.Email, false, 0, 0, 0, 0,
		// OAUTH2
		"**empty**", "**none**", "**invalid**", "**invalid**", 0,
		// 2FA
		"**invalid**", "**none**", "**none**", "")

	if err == nil {
		db.log.Printf("Successfully added user: GUID %s e-mail %s", user.GUID, user.Email)
	} else {
		db.log.Print(fmt.Errorf("Error adding user GUID %s/e-mail %s: %w", user.GUID, user.Email, err))
	}

	return err
}
