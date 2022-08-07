# Authboss. Worked. Code Walkthrough

- [Authboss. Worked. Code Walkthrough](#authboss-worked-code-walkthrough)
  - [HTML Templates](#html-templates)
    - [`html_templates.go`](#html_templatesgo)
    - [Template variables](#template-variables)
  - [`worked_udata.sqlite3`](#worked_udatasqlite3)
  - [The Code: The `abossworked` module](#the-code-the-abossworked-module)
    - [Gin vs. `http.Request` contexts](#gin-vs-httprequest-contexts)
    - [`config.go`](#configgo)
    - [`gormUData.go`](#gormudatago)
    - [`abossUData.go`](#abossudatago)
    - [authBoss.go](#authbossgo)
    - [ginRouter.go](#ginroutergo)

## HTML Templates

Authboss renders several different pages during the course of the user's
authentication, registration, recovery and management process. The documentation
refers to "templates"; more accurately, an Authboss template is the name of a
page that it needs to render. It just so happens that HTML templates are the
most sensible implementation.

### `html_templates.go`

This is where templates are loaded and rendered, and implements the
`authboss.Render` interface. The design and flow of control are simple:

- Pre-load all HTML templates via the `TemplateLoader()` function, which returns
  a `Templates *` object. `TemplateLoader()` walks the `content` subdirectory to
  load and parse a master template, fragment templates common to (or frequently
  used by) page templates and the page templates themselves. The `Template` type
  retains enough state to reload (hot load) all HTML templates when a filesystem
  change is detected.
  
  - Naming convention: 

    - The base name without the `.gohtml` extension is the name by which the
    page is referenced in the `Templates.templateMap` to retrieve the parsed
    HTML template.

    - Fragment templates in the `content/fragments` subdirectory have leading
      underscores (e.g., `_login_form`). Vide infra about fragment templates.

  - Master layout template (`master_layout.gohtml`)

    - This is the wrapper template with the HTML `html` and `body` tags and
    refers to a template named `content` for the actual page content.

    - Fragment templates (the `content/fragments` subdirectory) are parsed HTML
      templates nested within the master layout.  See the `_login_form`,
      `_logo_splash` and `_navbar` template fragments for examples -- these are
      both nested within the master template and can be referenced by the
      individual page templates:

      ````
      master_template -> _login_form
                         _logo_splash
                         _navbar
      ````

  - Individual page templates

    - Load and parse the `.gohtml` files in the top-level `content` directory.
 
    - For each parsed template, the master template is cloned and the parsed
    template nested within, with the name `content`.
    
      For example, the `index.gohtml` produces a cloned master template with the
      following nested content:

      ````
      master_template_clone -> _login_form
                               _logo_splash
                               _navbar
                               content      <- the parsed index.gohtml nested template
      ````

      `Templates.templateMap` stores the cloned master template using the naming
      convention described above:

      ````
      templates.templateMap["index"] = { master_template_clone -> _login_html
                                                               -> _logo_splash
                                                               -> _navbar
                                                               -> content
      }
      ````

- `Load` (Authboss interface) ensures that a requested Authboss HTML template
  page is loaded. You can see this in the log:

    ````
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: login present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: confirm_html present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: confirm_txt present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: recover_start present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: recover_end present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: recover_html present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: recover_txt present and accounted for.
    [CONFIG] 2022/08/06 12:59:35 Templates.Load: register present and accounted for.
    ````

    If `Load` doesn't find the named page in `Templates.templateMap`, it will
    log a _FATAL_ message and terminate `authboss-worked`.

- `Render` (Authboss interface) renders a named page for Authboss. This just
  retrieves the pages template from `Templates.templateMap` and calls
  `html.ExecuteTemplate`.

  _Note_: There are special templates that don't require rendering the master
  HTML layout. These are used for confirmation and account recovery e-mails,
  where Authboss crafts a MIME-encoded e-mail. See the `nonRenderedTemplates`
  map and its usage.


### Template variables

The `Render` interface function receives a `authboss.HTMLData` object, which is
a `map[string]string`. The `authboss.HTMLData` object stores associations
between variables referenced in the templates and their values. These
associations are gathered in a HTTP handler function and stored in both the Gin
and the `http.Request` contexts with the `authboss.CTXKeyData` key.

For example, `loggedin` is a boolean flag indicating whether the user is
currently logged in and used as a condition in several templates:

````
    {{ if not .loggedin }}
    <!-- show the login form -->
    {{ else }}
    <!-- stuff if the user is logged in -->
    {{ end }}
````

_NOTE_: `authboss.CTXKeyData` MUST BE USED as the context key if you want
Authboss to pass template variables to your code and HTML templates.


## `worked_udata.sqlite3`

`worked_udata.sqlite3` is the SQLite3 database backing store to the
`authboss-worked` demo. There are five tables that the demo creates and manages;
the `sesssions` table is created and managed by [Gin sessions][gin-sessions]
package.

|     Table        | Purpose    |
|:-----------------|:-----------|
| udata            | User GUID (primary key), Authboss primary identifier (e-mail) and _bcrypt_-ed password.
| confirmations    | User GUID (primary key, join to udata), confirmation selector, verifier and confirmation status (true/false)
| locked_accounts  | User GUID (primary key, join to udata), account lock status (attempts, last attempt time, lock expiration)
| recover_requests | User GUID (primary key, join to udata), recovery selector and verifier, and recovery token expiration
| remember         | User GUID (join to udata), "remember me" tokens. Should also have an expiration date/time (not implemented.)

The the `Create()` interface method in `abossUData.go` generates a GUID for the
new user, which is the primary key into the other four tables. The GUID
separates the reference to the user from the Authboss Primary Identifer
("_PID_"), which could be allowed to change. For example, this enables changing
the user's e-mail address at a later time, if you use the user's e-mail as their
primary identifier.

## The Code: The `abossworked` module

### Gin vs. `http.Request` contexts

Request handler functions receive a context parameter, which encapsulates useful
information related to the HTTP request, such as form data, query parameters,
HTTP header data, etc. Gin departs from the `http` package by providing its own
`gin.Context`, whereas other HTTP frameworks use the standard `context.Context`.

Authboss expects to retrieve data from the HTTP request's `context.Context` and
doesn't know anything about Gin's `gin.Context`. This means that if you update
or change data, you have to reinject it into the `http.Request` context:

````go
    func(ctx *gin.Context) {
        // Gin handler function: Stuff goes on, collects more template variables...

        // Update the http.Request's context
        ctx.Request = r.Clone(context.WithValue(r.Context(), authboss.CTXKeyData, abossCTXData))
        // Make the data available to downstream Gin handlers as well:
        ctx.Set(string(authboss.CTXKeyData), abossCTXData)
    }
````

### `config.go`

- YAML configuration reader: `GetWorkedConfig`
- YAML configuration writer: `GenerateWorkedConfig`
- The `ConfigData` structure contains all of the demo's configuration data,
  which is more than the YAML configuration. The YAML portion is embedded within
  `ConfigData`.

### `gormUData.go`

- Go structure types to [GORM][gorm.io] database mappings
- A couple of structures implement the `gorm.TableName()` interface to change
  the database table name for aesthetic reasons.

### `abossUData.go`

- `AuthStorer` contains the [GORM][gorm.io] connection.

- `WorkedUser` contains all of the state related to a user and user management: an embedded
  `AuthStorer` pointer and an embedded `UserState` structure.
  
  - The `AuthStorer` pointer enables the Authboss user-related interface
    functions (confirmations, lock status, ...) to query the database and avoids
    carrying that data in the `UserState` structure.

- `AuthStorer` specializes the `ServerStorer` and `CreatingServer` interfaces.
   whereas `WorkedUser` specializes the `ConfirmingServerStorer`,
  `RecoveringServerStorer`, `RememberingServerStorer` interfaces. 

  - `ServerStorer` and `CreatingServer` return an `authboss.User` &rarr; `WorkedUser`.

  - The other interfaces are get/put interfaces that operate directly on
    `WorkedUser` or structures related to `UserState`.

- The `ConfirmingServerStorer`, `RecoveringServerStorer`,
  `RememberingServerStorer` are intended to operate on structure members. The
  demo sends that data directly to the database as UPSERTs, which puts
  additional pressure on the underlying database server (SQLite3). There is a
  pattern to the sequence by which Authboss gets and puts data for these
  interfaces. However, it would be a suboptimal design to rely on those
  patterns. (_Note_: This is not a criticism of Authboss' design.)

- The `WorkedUser` specializations show to use a [GORM][gorm.io] subquery to
  join back to the `udata` table (`UserData` type) to get the user's GUID from
  their Authboss Primary Identifier (PID).

  - An example subquery used in the `ConfirmingServerStorer` (`subq` is the subquery):

    ````go
      subq := user.AuthStorer.UserDB.Model(&UserData{}).Select("guid").Where(&UserData{Email: user.GetPID()})
      result := user.AuthStorer.UserDB.Model(&Confirmations{}).
        Select("Confirmed").
        Where("GUID IN (?)", subq).
        First(&confirmed)
    ````

  - The alternative to using a subquery is an [Association][gorm-associations] join.

  - The demo doesn't generally use association joins, although the code is there
    to show how it is done. Association joins will fetch the entirety of the
    `UserData` structure from the `udata` table, which puts pressure on the
    SQLite database.

  - Association joins are used when the `WorkedUser` data is acutally needed,
    such as when retrieving the user via a confirmation or recovery selector
    (see `LoadByConfirmSelector` and `LoadByRecoverSelector`.)

  - Some enterprising developer will probably propose the following, "Why not
    avoid the subquery and just use the `WorkedUser` GUID directly?" Yes, that's
    a valid optimization, but doesn't check the constraint that the user's
    primary identifier exists in the `udata` table. The subquery is there for
    sanity checking. You can't be too paranoid when it comes to ensuring data
    integrity (while keeping that integrity overhead to a minimum.)

- [GORM][gorm.io] has an implementation concept called `gorm.Model`, which is
  normally embedded in structures. 

  - `gorm.Model` is not strictly required, and the demo only uses a subset of
    the `gorm.Model`'s members. 

  - Specifically, the demo does not use the `DeletedAt` member: `DeletedAt`
    implements a "soft delete" feature. "Soft delete" doesn't actually delete
    data; all queries have an extra condition that selects "active" rows where
    `DeletedAt` is _NOT NULL_.

### authBoss.go

- This is where Authboss configuration happens: `configureAuthboss` &rarr; `(ab *authboss.Authboss, err error)`

- Authboss configuration has two phases: `default.SetCore()` and `HTTPBodyReader`.

  - Initialize `authboss.Config` members before invoking `default.SetCore()`.
    Technically speaking, all you really need to do is set the renderers
    `Core.ViewRenderer` and `Core.MailRenderer`. However, if `default.SetCore()`
    ever changes and requires more `authboss.Config` initialization, the changes
    won't break your code (much.)

  - `HTTPBodyReader` is the default implementation that reads JSON or form data
    and does input validation. The forms have requirements that are lightly
    documented, if documented at all.

    - It looks like the form field names are flexible, but from reading the code, that's well, maybe&hellip;

    - `default.NewHTTPBodyReader()` takes two boolean parameters: (readJSON, useUsernameNotEmail bool)`. 

      - `readJSON` &larr; `false` to read HTML form data, `true` if reading JSON.

      - `useUsernameNotEmail` &larr; false if using e-mail addreses as the
        user's Primary Identifier (PID) or `true` if you have a user naming
        convention.

    - Form requirements (refer to the HTML templates for where they are used, when they are used):

      | Form field | Purpose |
      |:-----------|:--------|
      | name | The user name, if `useUsernameNotEmail` is true. If not using user names, don't include this field in your form. |
      | email | The user's e-mail. The form still requires this field even when `useUsernameNotEmail` is true, for confirmation and account recovery |
      | password | The user's password. |
      | confirm_password | Password confirmation during user registration. This must match the password field. |
      | rm | "Remember Me" checkbox's value. This is hard-coded and will drive you nuts until you read the Authboss code! |

    - Data validation: There are six (6) form data validation rulesets
      (`map[string][]Rules`), and two default `authboss.Rule{}`-s, `pidRule` and
      `passwordRule`. Both rules specify requirements for certain form data,
      such as "must be present", minimum/maximum length, regular expression
      pattern matching.

      | Page name | Rules | Explanation |
      |:----------|:------|:------------|
      | login | pidRule | Ensures that the primary identifier is present in the form (user name or e-mail) |
      | register | pidRule, passwordRule | Validates primary identifier and password format for user registration |
      | confirm | --- | Ensures that a field named "cnf" exists during the confirmation process (hidden form field?) |
      | recover_start | pidRule | Validates the primary identifier's format to recover a user's account |
      |recover_end | passwordRule | Validates the password's format when changing a user's password at the end of account recovery. |
      | twofactor_verify_end | --- | Ensures that a field named "token" exists at the end of the 2FA process (hidden form field?) |

      - `authboss.Rule{}` has a field name member, which identifies the data to
        be validated. And that data does get validated.

    - If you want to use different form field names or have more complicated
      data validation rulesets or some other requirement that `HTTPBodyReader`
      doesn't address, you will have to roll your own version of
      `HTTPBodyReader` and implement the `Read` interface, as well as sundry
      other interfaces invoked during data validation.

    - __UTSL__[^1] even if you don't need to customize `HTTPBodyReader`.
      `HTTPBodyReader` most likely covers 80-90% of the authentication use
      cases, but it's still instructive to walk through the code.

    - The demo relaxes the password requirements from the default. (_UTSL_ for
      the default password requirements.)


### ginRouter.go

- This file has three parts: the Gin router (engine) configuration, the session
  store and the cookie store.

- Authboss and Gorilla CSRF middleware functions have to be "wrapped" so that
  they conform to the Gin handler function interface.

  - The "standard" Gin wrapper package does not expose the inner
    `swappedResponseWriter` structure. Authboss requires an
    `UnderlyingResponseWriter()` interface implementation to acquire a HTTP
    response writer object. Pretty tricky to do when `swappedResponseWriter`
    isn't exported.

  - To work around this problem, `abossworked.adapter` is a local copy of the
    standard Gin wrapper package with an `UnderlyingResponseWriter()` interface
    implementation.

- Gin router (engine) configuration

  - Decode the session, cookie and CSRF seeds from their Base64 encoding.
  
  - Create the session and cookie stores.

  - Create the `authboss.Authboss` authentication object (vide supra)

  - Configure the middleware stack at the web server's root (_"/"_):

    - Mandatory, in this order. These really do need to come before other
      middleware executes.
      - Gorilla CSRF
      - Gin session cookie management
      - `authboss.LoadClientStateMiddleware`

    - Conditionally add the Authboss "Remember me" middleware module.

    - Template variable data collection

  - Route _"/auth"_ into Authboss. Note that the _"/auth"_ prefix has to be
    stripped or Authboss won't recognize its own routes (see
    `authboss.Config.Paths.Mount`). This follows the original sample code's
    methdology.

  - Configure the _"/app"_ route, which is the part of the web server's name space
    we want protected

    - `authboss.Middleware2()` is the function that injects all of the Authboss
      middleware needed to protect a portion of the web server's name space.

    - If enabled in the configuration, only allow confirmed users and users with
      unlocked accounts to access the "/app" name space.

    - The `app_index` HTML template renders the _"/app/"_ index page.

  - Add handlers for additional pages and static content (images).

  - Return the resulting `gin.Engine` object pointer, go off and do good things.

- The session store retrieves the user's session data from the `http.Request`'s
  context and packages it in a `SessionState` structure, from which Authboss
  reads session data.
  
  - This is the driving reason why session middleware handlers have to execute
    before `authboss.LoadClientStateMiddleware`. We need an updated session
    store in order to extract up-to-date data.

  - The `WriteState` interface method is where Authboss makes updates to session
    state. `WriteState` doesn't write the session cookie into the
    `http.ResponseWriter`; we let the GORM session store manage that for us.

- Cookie manager

The cookie manager reads and writes Authboss cookies that should be included
with every request (alternatively, should persist acrosss requests.) The only
cookie that Authboss needs to persist across requests is the "Remember me"
cookie, `rm`.

Application and session cookies are handled separately.


[^1]: See the [Jargon File entry](http://www.catb.org/jargon/html/U/UTSL.html)

[gorm.io]: https://gorm.io/
[gin-sessions]: https://github.com/gin-contrib/sessions
[gorm-associations]: https://gorm.io/docs/associations.html