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
	"log"
	"regexp"
	"time"

	"github.com/volatiletech/authboss/v3"
	"github.com/volatiletech/authboss/v3/defaults"

	// All of the modules that we intend to use, even if we don't use them in the
	// web framework router.
	//
	// NOTE: Don't try to pass a list of module names to authboss.Init() if you reference
	// the module elsewhere in your code. This is a Path of Tears -- authboss will register
	// the module internally as being needed, ignoring your requested list.
	_ "github.com/volatiletech/authboss/v3/auth"
	_ "github.com/volatiletech/authboss/v3/confirm"
	_ "github.com/volatiletech/authboss/v3/lock"
	_ "github.com/volatiletech/authboss/v3/logout"
	_ "github.com/volatiletech/authboss/v3/recover"
	_ "github.com/volatiletech/authboss/v3/register"
)

// configureAuthboss initializes an authboss.Authboss entity.
func configureAuthboss(cfg *ConfigData, sessionStore *SessionStore, cookieStore *CookieStorer, templates *Templates,
	storer *AuthStorer) (ab *authboss.Authboss, err error) {
	ab = authboss.New()

	// The default for the LogoutMethod is "DELETE", which requires some JavaScript to invoke
	// as the HTTP method. Change it to "GET" for the Logout button.
	ab.Config.Modules.LogoutMethod = "GET"
	// Redirect to the login page when unauthorized. This is a default and probably not useful
	// since authboss.Middleware2()'s third parameter [failureResponse] overrides this completely.
	// Nonetheless, provide something reasonable.
	ab.Config.Modules.ResponseOnUnauthed = authboss.RespondRedirect

	ab.Config.Paths.RootURL = "http://" + cfg.HostPortString()
	ab.Config.Storage.Server = storer
	ab.Config.Storage.SessionState = sessionStore
	ab.Config.Storage.CookieState = cookieStore

	// The URL prefix for Authboss' URL namespace.
	ab.Config.Paths.Mount = "/auth"

	/* Redirection paths: Authboss generates HTTP redirects to your pages once it completes
	   an action, such as registration. Modify the URL paths as needed -- the paths below were
	   copied and pasted from Authboss' defaults.
	*/
	ab.Config.Paths.NotAuthorized = "/unauthorized"
	ab.Config.Paths.AuthLoginOK = "/app/"
	ab.Config.Paths.ConfirmOK = "/"
	ab.Config.Paths.ConfirmNotOK = "/"
	ab.Config.Paths.LockNotOK = "/"
	ab.Config.Paths.LogoutOK = "/logout"
	ab.Config.Paths.OAuth2LoginOK = "/"
	ab.Config.Paths.OAuth2LoginNotOK = "/"
	ab.Config.Paths.RecoverOK = "/"
	ab.Config.Paths.RegisterOK = "/"
	ab.Config.Paths.TwoFactorEmailAuthNotOK = "/"

	// This is the connection between Authboss and YOUR HTML, when Authboss needs to render a form for
	// self-registration or login. Each module ("auth", "register", "recover", ...) has its own path
	// and needs to support GET and POST methods in your favorite Web framework's router.
	ab.Config.Core.ViewRenderer = templates

	// Use the same template renderer for the MailRenderer.
	ab.Config.Core.MailRenderer = templates

	// Preserve the email and name fields during user registration (prevents having
	// to type them again)
	ab.Config.Modules.RegisterPreserveFields = []string{"email", "name"}

	// Defaults for locking: 3 attempts, lockout for 5 minutes, reset the attempt
	// count after 3 minutes.
	ab.Config.Modules.LockAfter = 3
	ab.Config.Modules.LockDuration = time.Duration(5) * time.Minute
	ab.Config.Modules.LockWindow = time.Duration(3) * time.Minute

	// defaults.SetCore() has to be called to set up Authboss internals.
	defaults.SetCore(&ab.Config, false, false)

	/* READ THE CODE in authboss/defaults/values.go.

	   HTTPBodyReader and its constructor, NewHTTPBodyReader(), define which
	   form fields are validated, extracted and mapped to User structure members.

	   The most important of these is the PID, which uniquely identifies the user.
	   NewHTTPBodyReader() defaults to the "email" field, or the "username" field
	   if passed useUsernameNotEmail=true. In this example, the PID is the email
	   address.

	   If you use the user name approach, then the call to defaults.NewHTTPBodyReader()
	   should be:

	       bodyReader := defaults.NewHTTPBodyReader(false, true)

	   There are five validation rulesets:	"login", "register", "confirm", "recover_start",
	   "recover_end".

	   If you use the user name approach, you so want to validate the user name via nameRule
	   when registering the user:

	       bodyReader.Rulesets["register"] = []defaults.Rules{emailRule, passwordRule, nameRule}
	*/
	bodyReader := defaults.NewHTTPBodyReader(false, false)

	emailRule := defaults.Rules{
		FieldName:  "email",
		Required:   true,
		MatchError: "Must be a valid e-mail address",
		MustMatch:  regexp.MustCompile(`.*@.*\.[a-z]{1,}`),
	}
	passwordRule := defaults.Rules{
		FieldName:  "password",
		Required:   true,
		MatchError: "Password is required.",
		MinLength:  4,
	}
	/* Validation check for user names, as an alternative to e-mail as the primary
	   authboss UID. Uncomment this validation rule if you also uncomment the
	   "name" field in the index.gohtml login form. */
	/* nameRule := defaults.Rules{
	    FieldName:  "name",
	    Required:   true,
	    MatchError: "User name is required.",
	    MinLength:  2,
	} */

	// Validate the email address, password. Uncomment the nameRule if you uncomment the
	// "name" field in the index.gohtml form and nameRule validation rule.
	bodyReader.Rulesets["register"] = []defaults.Rules{emailRule, passwordRule /*, nameRule */}
	// Recovery: Just validate the password
	bodyReader.Rulesets["recover_end"] = []defaults.Rules{passwordRule}

	// Form fields that the confirm module will inspect to ensure that passwords match
	// for the register and recover modules.
	bodyReader.Confirms["register"] = []string{"password", authboss.ConfirmPrefix + "password"}
	bodyReader.Confirms["recover_end"] = []string{"password", authboss.ConfirmPrefix + "password"}

	bodyReader.Whitelist["register"] = []string{"email", "name", "password"}

	ab.Config.Core.BodyReader = *bodyReader

	// Note: Don't

	if err := ab.Init(); err != nil {
		// Handle error, don't let program continue to run
		log.Fatalln(err)
		return nil, err
	}

	return ab, nil
}
