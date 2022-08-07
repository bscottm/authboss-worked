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
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	gsessions "github.com/gin-contrib/sessions"
	// If you want cookie-based session storage, uncomment the line below
	// gcookie "github.com/gin-contrib/sessions/cookie"
	//
	// Example code uses GORM-based SQLite:
	gsqlite "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"github.com/gorilla/securecookie"
	errmgmt "github.com/pkg/errors"
	"github.com/volatiletech/authboss/v3"

	"github.com/volatiletech/authboss/v3/confirm"
	"github.com/volatiletech/authboss/v3/lock"
	"github.com/volatiletech/authboss/v3/remember"

	// Remove this import and the adapter subdirectory if and when the gin adapter
	// package exposes the swappedResponseWriter field. This is supposed to be a
	// temporary workaround.
	"gitlab.com/scooter-phd/authboss-worked/abossworked/adapter"

	// Pretty printer
	"github.com/davecgh/go-spew/spew"
)

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// Gin router setup:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// GinRouter configures the Gin framework's URL dispatching and routing.
func GinRouter(cfg *ConfigData, storer *AuthStorer, templates *Templates) (engine *gin.Engine, err error) {
	sessionSeed, err := base64.StdEncoding.DecodeString(cfg.yamlConfig.Seeds.SessionSeed)
	if err != nil {
		return nil, fmt.Errorf("unable to decode session seed: %w", err)
	}

	cookieSeed, err := base64.StdEncoding.DecodeString(cfg.yamlConfig.Seeds.CookieSeed)
	if err != nil {
		return nil, fmt.Errorf("unable to decode cookie seed: %w", err)
	}

	csrfSeed, err := base64.StdEncoding.DecodeString(cfg.yamlConfig.Seeds.CSRFSeed)
	if err != nil {
		return nil, fmt.Errorf("unable to decode CSRF seed: %w", err)
	}

	logger := log.New(os.Stdout, "[ABOSSWORKED] ", log.LstdFlags)
	sessionStore := makeSessionStore(storer, sessionCookieParams, sessionCookieName, sessionSeed, nil)

	cookieStore := makeCookieStorer(cookieSeed, nil, logger)
	cookieStore.HttpOnly = false
	cookieStore.Secure = false

	var aboss *authboss.Authboss

	aboss, err = configureAuthboss(cfg, sessionStore, cookieStore, templates, storer)
	if err != nil {
		return nil, err
	}

	// Gin Gonic setup:
	engine = gin.New()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.SetTrustedProxies(nil)

	// Setup middleware (functions invoked before the handler)

	// Construct a slice of middlware handlers that are subsequently passed en masse
	// to router.Use(). This lets us conditionally add middlware that has to be invoked
	// in a specific order.

	middleware := []gin.HandlerFunc{
		// CSRF protection via Gorilla:
		adapter.Wrap(csrf.Protect(csrfSeed,
			// In a production environment, you should use csrf.Secure(true)
			csrf.Secure(false),
			// And a more robust error handler:
			csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message": "Forbidden - CSRF token invalid"}`))
			})),
		)),

		// Gin-contrib session middleware: This acquires the session data and puts it into the
		// gin context, after which you can work with the session via its Set() and Get()
		// interface functions.
		gsessions.Sessions(sessionCookieName, sessionStore.gstore),

		// Authboss doesn't know about the Gin context, so you need this lambda to run
		// after the gsessions.Sessions() handler to hoist the session data into the
		// request's http.Context.
		//
		// Also, add the CSRF token to the response header (instead of adding a new Use() lambda.)
		func() gin.HandlerFunc {
			return func(ctx *gin.Context) {
				sessionData := gsessions.Default(ctx)
				ctx.Request = ctx.Request.Clone(context.WithValue(ctx.Request.Context(), httpSessionKey, sessionData))

				// Add CSRF token to the HTTP header. This is likely redundant in this example because
				// we add it to the form. If you use JavaScript or JSON, you would want the CSRF token
				// in the header. (See https://github.com/gorilla/csrf#javascript-applications)
				//
				// No, you probably don't have to add a CSRF token to every single request. It doesn't hurt
				// if you do. It's likely overkill, though.
				ctx.Writer.Header().Set("X-CSRF-Token", csrf.Token(ctx.Request))
			}
		}(),

		// Session state code MUST INVOKE before aboss.LoadClientStateMiddleWare handlers, or you
		// will never log the user into your system.

		// aboss.LoadClientStateMiddleware: This is the critical "must load" middleware for AuthBoss:
		adapter.Wrap(aboss.LoadClientStateMiddleware),
	}

	// Conditionally add "Remember me" just after authboss.LoadClientStateMiddleWare()
	if cfg.yamlConfig.Features.UseRemember {
		middleware = append(middleware, adapter.Wrap(remember.Middleware(aboss)))
	}

	middleware = append(middleware,
		// Grab list of modules in the request's context for template rendering.
		adapter.Wrap(authboss.ModuleListMiddleware(aboss)),

		// Collect the variables used to drive the templates, adding the authboss.HTMLData
		// map to the http.Request context. The http.Request context IS NOT THE SAME AS
		// the Gin context!
		func() gin.HandlerFunc {
			return func(ctx *gin.Context) {
				// Collect the data used within templates:
				currentUserName := ""
				r := ctx.Request
				w := ctx.Writer
				// Closure with aboss:
				currentUser, err := aboss.LoadCurrentUser(&r)
				if currentUser != nil && err == nil {
					currentUserName = currentUser.(*WorkedUser).Email
				}

				// Authboss may have already created some data for us (the module list),
				// so don't wipe it out:
				var abossCTXData authboss.HTMLData

				ctxData := r.Context().Value(authboss.CTXKeyData)
				if ctxData != nil {
					abossCTXData = ctxData.(authboss.HTMLData)
				} else {
					abossCTXData = authboss.HTMLData{}
				}

				abossCTXData["loggedin"] = (currentUser != nil)
				abossCTXData["current_user_name"] = currentUserName
				abossCTXData[csrf.TemplateTag] = csrf.TemplateField(ctx.Request)
				abossCTXData["flash_success"] = authboss.FlashSuccess(w, r)
				abossCTXData["flash_error"] = authboss.FlashError(w, r)
				abossCTXData["feature_remember"] = cfg.Features.UseRemember

				// Grab the recovery token if it's present (usually in the query string), make it
				// available in the template renderer. Use Gin's BindQuery method to add the "token"
				// to the HTMLData.
				if recoveryToken, tokenPresent := ctx.GetQuery("token"); tokenPresent {
					abossCTXData["recovery_token"] = recoveryToken
				}

				// Make the HTMLData available to both Gin and Authboss:
				ctx.Set(string(authboss.CTXKeyData), abossCTXData)
				ctx.Request = r.Clone(context.WithValue(r.Context(), authboss.CTXKeyData, abossCTXData))
			}
		}(),
	)

	engine.Use(middleware...)

	/* Route the entirety of the "/auth" namespace to authboss. There are two ways of doing
	   this in Gin, both of which use wildcard paths. You could use Group(), like this:

	        authGroup := router.Group("/auth")
	        {
	                authGroup.Any("/*wild", gin.WrapH(http.StripPrefix("/auth", aboss.Config.Core.Router)))
	        }

	        Not clear whether this is advantageous as compared to calling Any() with the full path
	        and the wildcard.

	        Either way, you still have to strip the "/auth" prefix for authboss to recognize its own
	        paths within the "/auth" namespace. This is a consequence of the .Config.Paths.Mount
	        setting.
	*/
	engine.Any("/auth/*wild", gin.WrapH(http.StripPrefix("/auth", aboss.Config.Core.Router)))

	/* Pull all of the /app middleware together: */
	appMiddleware := []gin.HandlerFunc{
		// Note: authboss.RespondRedirect overrides the configuration's default.
		adapter.Wrap(authboss.Middleware2(aboss, authboss.RequireFullAuth, authboss.RespondRedirect)),
	}

	// Add the confirm module to /app so only confirmed users can access the namespace.
	if cfg.yamlConfig.Features.UseConfirm {
		appMiddleware = append(appMiddleware, adapter.Wrap(confirm.Middleware(aboss)))
	}

	// Add the lock module:
	if cfg.yamlConfig.Features.UseLock {
		appMiddleware = append(appMiddleware, adapter.Wrap(lock.Middleware(aboss)))
	}

	/* Route the application's namespace as a group. */
	appspace := engine.Group("/app")
	appspace.Use(appMiddleware...)
	appspace.GET("/", renderPageAsTemplate("app_index", templates))
	appspace.GET("/user", renderPageAsTemplate("app_user", templates))
	appspace.POST("/user", userManagementPost(aboss))

	// Static content:
	engine.StaticFS("/images", http.Dir("content/images"))

	// "Logout" page:
	engine.GET("/logout", renderPageAsTemplate("logout", templates))
	// "Unauthorized" page:
	engine.GET("/unauthorized", renderPageAsTemplate("login", templates))
	// Root page:
	engine.GET("/", renderPageAsTemplate("index", templates))

	return engine, nil
}

func renderPageAsTemplate(pageName string, templateCatalog *Templates) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		r := ctx.Request
		w := ctx.Writer

		var result []byte
		var contentType string
		var err error
		var ctxData = r.Context().Value(authboss.CTXKeyData).(authboss.HTMLData)

		if result, contentType, err = templateCatalog.Render(r.Context(), pageName, ctxData); err == nil {
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			w.Write(result)
		} else {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprint(fmt.Errorf("template render error: %v", err))))
		}
	}
}

func userManagementPost(aboss *authboss.Authboss) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentPassword, currentPresent := ctx.GetPostForm("current_password")
		newPassword, newPresent := ctx.GetPostForm("password")
		confirmPassword, confirmPresent := ctx.GetPostForm("confirm_password")

		doRedirect := false
		errMessage := ""

		switch {
		case !currentPresent || len(currentPassword) == 0:
			errMessage = "invalid or missing current password"
			doRedirect = true
		case !newPresent || len(newPassword) == 0:
			errMessage = "invalid or missing new password"
			doRedirect = true
		case !confirmPresent || len(confirmPassword) == 0:
			errMessage = "invalid or missing confirm password"
			doRedirect = true
		case newPassword != confirmPassword:
			errMessage = "new password does not match confirm password"
			doRedirect = true
		default:
			currentUser, err := aboss.LoadCurrentUser(&ctx.Request)
			if currentUser != nil && err == nil {
				if user, validUser := currentUser.(*WorkedUser); validUser {
					if authboss.VerifyPassword(user, currentPassword) != nil {
						errMessage = "current password did not verify."
						doRedirect = true
					} else {
						// Sanity checks passed, user's current password verifies with what we have in the
						// database...

						// 1. Update the password, kill current session and cookies:
						aboss.UpdatePassword(ctx.Request.Context(), user, newPassword)
						authboss.DelAllSession(ctx.Writer, []string{})
						authboss.DelKnownCookie(ctx.Writer)

						// 2. Send them to the root page.
						ctx.Redirect(http.StatusFound, "/")
						return
					}
				} else {
					errMessage = "type annotation to WorkedUser failed (??)"
					doRedirect = true
				}
			} else {
				errMessage = "unable to determine your user name or info (??)"
				doRedirect = true
			}
		}

		if doRedirect {
			authboss.PutSession(ctx.Writer, authboss.FlashErrorKey, errMessage)
			ctx.Redirect(http.StatusFound, ctx.Request.URL.String())
		}
	}
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// Session management:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

const (
	sessionCookieName string = "abossworked_session"
	// 30 minute session ages:
	sessionMaxAge = int(30 * time.Minute / time.Second)
)

// HTTP session key type. goLint suggests using something other than a string type as the
// key type when hoisting data into the http.Context to avoid naming collisions.
type httpSessionKeyType struct {
	sessionUID  int
	sessionName string
}

var (
	sessionCookieParams = gsessions.Options{
		// NOTE: Make sure you set the cookie's path to something sensible. "/"
		// covers the entirety of your web server's namespace. You could set the
		// session cookie's path to another part of your web server's URl namespace,
		// e.g., "/webapp" (although this may impact how Authboss ultimately works,
		// and you may need to experiment a bit.)
		Path:     "/",
		Domain:   "",
		MaxAge:   sessionMaxAge,
		Secure:   false,
		HttpOnly: false,
		SameSite: http.SameSiteDefaultMode,
	}

	httpSessionKey = httpSessionKeyType{
		sessionUID:  0xf00f,
		sessionName: gsessions.DefaultKey,
	}
)

// SessionState is the Gin-wrapped Gorilla session container using GORM-based storage.
type SessionState struct {
	logger       *log.Logger
	gSessionData gsessions.Session
}

// Get a key from the session
func (s SessionState) Get(key string) (string, bool) {
	gSessionValue := s.gSessionData.Get(key)
	if gSessionValue == nil {
		return "", false
	}

	gValue, gOk := gSessionValue.(string)
	s.logger.Printf("SessionState.Get(%v) -> %v %v", key, gValue, gOk)

	return gValue, gOk
}

// SessionStore stores sessions in a Gin-contrib, GORM-backed session store.
type SessionStore struct {
	Name     string
	logger   *log.Logger
	pprinter *spew.ConfigState
	gstore   gsessions.Store
}

// makeSessionStore creates a new Gin-contrib, GORM-backed session store that also
// implements the interface functions for Authboss.
//
// The keypairs arguments are (HMAC, encryption) key pairs. See github.com/gorilla/securecookie
// for more complete documentation.
//
// The code currently uses the Gorm-based, SQLite3-backed database for session storage.
// Uncomment the cookie store constructor code and update to return gcookieStore instead
// of gsqliteStore.
func makeSessionStore(storer *AuthStorer, defaultCookieParams gsessions.Options, sessionName string, keypairs ...[]byte) *SessionStore {
	/* gcookieStore := gcookie.NewStore(keypairs...)
	   gcookieStore.Options(defaultCookieParams) */

	gsqliteStore := gsqlite.NewStore(storer.UserDB, true, keypairs...)
	gsqliteStore.Options(defaultCookieParams)

	pprinter := spew.NewDefaultConfig()
	pprinter.Indent = "  "
	pprinter.SortKeys = true

	return &SessionStore{
		Name:     sessionName,
		logger:   log.New(os.Stdout, "[SESSION] ", log.LstdFlags),
		pprinter: pprinter,
		gstore:   gsqliteStore,
	}
}

// ReadState loads the session from the http.Request context
func (s SessionStore) ReadState(r *http.Request) (authboss.ClientState, error) {
	ctx := r.Context()
	if ctx == nil {
		return nil, errors.New("nil request context in ReadState()")
	}

	gsValue := ctx.Value(httpSessionKey)
	if gsValue == nil {
		return nil, errors.New("nil gsValue in ReadState()")
	}

	return &SessionState{
		logger:       s.logger,
		gSessionData: gsValue.(gsessions.Session),
	}, nil
}

// WriteState to the responsewriter
func (s SessionStore) WriteState(w http.ResponseWriter, state authboss.ClientState, ev []authboss.ClientStateEvent) error {
	ses := state.(*SessionState)
	for _, ev := range ev {
		switch ev.Kind {
		case authboss.ClientStateEventPut:
			ses.gSessionData.Set(ev.Key, ev.Value)
			s.logger.Printf("WriteState(%s): %v -> %v", s.Name, ev.Key, ev.Value)

		case authboss.ClientStateEventDel:
			ses.gSessionData.Delete(ev.Key)
			s.logger.Printf("WriteState(%s): %s deleted.", s.Name, ev.Key)

		case authboss.ClientStateEventDelAll:
			if len(ev.Key) == 0 {
				// Delete the entire session
				ses.gSessionData.Options(gsessions.Options{
					MaxAge: -1,
				})
			} else {
				/* The bummer here is that gsessions.Session doesn't have an
				   interface function to grab the Values map. Save all of the
				   values that are whitelisted, clear the session's internal map,
				   then put the saved values back.*/
				whitelist := strings.Split(ev.Key, ",")
				saveWhitelist := make(map[string]interface{}, len(whitelist))

				for _, key := range whitelist {
					saveWhitelist[key] = ses.gSessionData.Get(key)
				}

				ses.gSessionData.Clear()

				for key, value := range saveWhitelist {
					ses.gSessionData.Set(key, value)
				}
			}
		}
	}

	return ses.gSessionData.Save()
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// Cookie management:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

const (
	// 12 hours in seconds
	defaultCookieAge = int(12 * time.Hour / time.Second)
)

// CookieState is an authboss.ClientState implementation to hold
// cookie state for the duration of the request
type CookieState map[string]string

// Get a cookie
func (c CookieState) Get(key string) (string, bool) {
	cookie, ok := c[key]
	return cookie, ok
}

var (
	// Default list of legitimate cookies in which we're interested.
	defaultCookieList = []string{
		authboss.CookieRemember,
	}
)

// CookieStorer writes and reads cookies to an underlying gorilla secure cookie
// storage.
type CookieStorer struct {
	// Legitimate cookies in which we're interested
	Cookies []string
	// Default cookie parameters (age, same site, domain, path, ...). Only a subset
	// of parameters are used.
	http.Cookie
	// Embedded secure cookie storage and management.
	*securecookie.SecureCookie
	// Logger
	logger *log.Logger
}

// makeCookieStorer creates a new cookie storer. These cookies are ephemeral,
// only alive for the lifetime of the application.
func makeCookieStorer(hashKey, blockKey []byte, logger *log.Logger) *CookieStorer {
	return &CookieStorer{
		Cookies: defaultCookieList,
		Cookie: http.Cookie{
			Path:     "/",
			MaxAge:   defaultCookieAge,
			HttpOnly: false,
			Secure:   false,
		},
		SecureCookie: securecookie.New(hashKey, blockKey),
		logger:       logger,
	}
}

// ReadState from the request
func (c CookieStorer) ReadState(r *http.Request) (authboss.ClientState, error) {
	cs := make(CookieState)

	for _, cookie := range r.Cookies() {
		for _, n := range c.Cookies {
			if n == cookie.Name {
				var str string
				if err := c.SecureCookie.Decode(n, cookie.Value, &str); err != nil {
					if e, ok := err.(securecookie.Error); ok {
						// Ignore bad cookies, this means that the client
						// may have bad cookies for a long time, but they should
						// eventually be overwritten by the application.
						if e.IsDecode() {
							continue
						}
					}
					return nil, err
				}

				c.logger.Printf("CookieStorer.ReadState: %v -> %s", n, str)
				cs[n] = str
			}
		}
	}

	return cs, nil
}

// WriteState to the responsewriter
func (c CookieStorer) WriteState(w http.ResponseWriter, state authboss.ClientState, ev []authboss.ClientStateEvent) error {
	for _, ev := range ev {
		switch ev.Kind {
		case authboss.ClientStateEventPut:
			encoded, err := c.SecureCookie.Encode(ev.Key, ev.Value)
			if err != nil {
				return errmgmt.Wrap(err, "failed to encode cookie")
			}

			c.logger.Printf("CookieStorer.WriteState: Put %v -> %v", ev.Key, ev.Value)
			http.SetCookie(w, &http.Cookie{
				Expires: time.Now().UTC().AddDate(1, 0, 0),
				Name:    ev.Key,
				Value:   encoded,

				Domain:   c.Domain,
				Path:     c.Path,
				MaxAge:   c.Cookie.MaxAge,
				HttpOnly: c.HttpOnly,
				Secure:   c.Secure,
				SameSite: c.SameSite,
			})
		case authboss.ClientStateEventDel:
			c.logger.Printf("CookieStorer.WriteState: Delete %v", ev.Key)
			http.SetCookie(w, &http.Cookie{
				MaxAge: -1,
				Name:   ev.Key,
				Domain: c.Domain,
				Path:   c.Path,
			})
		}
	}

	return nil
}
