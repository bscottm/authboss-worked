# Authboss -- A Worked Example (for Gin-Gonic)

<!-- TOC -->

- [Authboss -- A Worked Example (for Gin-Gonic)](#authboss----a-worked-example-for-gin-gonic)
    - [GPLv3 License](#gplv3-license)
    - [Disclaimer](#disclaimer)
        - [Why this code isn't a skeleton](#why-this-code-isnt-a-skeleton)
    - [Why?](#why)
    - [Running the demo](#running-the-demo)
        - [Generate/create the YAML configuration file](#generatecreate-the-yaml-configuration-file)
            - [Seed values](#seed-values)
        - [Run the demo](#run-the-demo)
        - [Try the demo's functionality](#try-the-demos-functionality)
            - [Sign in as a nonexistent user](#sign-in-as-a-nonexistent-user)
            - [Register and confirm a user](#register-and-confirm-a-user)
            - [Sign in as a valid user.](#sign-in-as-a-valid-user)
            - [Lockout](#lockout)
            - [Recover an account](#recover-an-account)
    - [Where to go from here...](#where-to-go-from-here)
    - [What does "scooter me fecit" mean?](#what-does-scooter-me-fecit-mean)

<!-- /TOC -->

## GPLv3 License

"scooter me fecit"

Copyright 2022 B. Scott Michel

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program in the [COPYING.md](COPYING.md) file. If not, see [The GNU Public
Licenses][gpl-licenses].

## Disclaimer

This program is a "_HOWTO_" guide: it shows you how to use certain functionality
available in the Authboss authentication management package. It is not intended
to be a "code skeleton" and should be treated as prototype demonstration code.

### Why this code isn't a skeleton

* The authboss-worked web server uses HTTP, not HTTPS. You would need to add the
  scaffolding to support HTTPS (certificates, keys, etc.)
* The ```worked_udata.sqlite3``` database contains "sensitive" user data,
  notably their password. Even though `bcrypt` is considered very robust, you
  would want to ensure that your sensitive user data files or table data is
  encrypted-at-rest.
* SQLite is not a good choice as an authentication backing store in a
  multi-instance application deployment environment because it doesn't scale
  (doesn't replicate easily, for one.)
* A good design separates the application data and authentication databases
  so that sensitive data doesn't comingle with less sensitive data.
* This code tries to abide by proper separation of concerns, but has never
  had a security audit.

## Why?

[Authboss][authboss] has very decent reference documentation, once you get to
know Authboss' principles of operation and quirks. The [Authboss sample
code][authboss_sample] showing how to use Authboss' features is helpful, but not
self-contained -- you have to refer to several different Github repos to
understand the flow of control, for example. The sample code uses the
[chi][go-chi] web application framework; a frequently asked question is, "Is
there a sample that works with [gin-gonic][gin-gonic]?"

The existing [gin-gonic Authboss example][authboss-gin-sample] shows how to wrap
`http.Handler` functions so they work with Gin as middleware. The code type
checks and compiles, but doesn't actually have any functionality demonstrating
the glue between Authboss and Gin-gonic. Basically, the existing gin-gonic
example isn't particularly helpful.

This example code attempts to provide a "_HOWTO_" reference that glues together
several components commonly found in web applications:

- Golang [HTML templates][html.template],

  The HTML templates are self-contained within the code base, so there are no
  references to external projects and skipping around while trying to figure out
  flow of control.

- The [GORM (Golang Object Relational Model)][gorm.io],

  The backing store for this example is the [GORM SQLite][gorm-sqlite] driver,
  used to store various parts of Authboss state (e.g., user data, confirmations,
  account lockouts, recovery requests.)

- [Gin session middleware][gin-sessions],

  The [Gorilla session middleware][gorilla-sessions] could be used "out of the
  box" for Gin session management. Undoubtedly, someone will eventually try to
  use the [Gin session middleware][gin-sessions] and get stuck trying to get it
  to work with Authboss.

- The [Gin-gonic][gin-gonic] web framework, and

- [YAML][go-yaml]-based configuration.


## Running the demo

### Generate/create the YAML configuration file

The Authboss demo uses the `data/config/worked-config.yml` YAML configuration
file to set a few parameters, such as the web server's listening address. The
fully commented YAML configuration template file is
`data/config/worked-config.template.yml`.


The `genconfig/genconfig.go` will helpfully create a starting configuration file
for you.

````
$ go run genconfig/genconfig.go
[CONFIG] 2022/07/21 12:22:55 Generated session seed: bd5c0b3745d2291fbdc0ea177911e7bad071e4b0c746636d74d607d7364cb1ab54219a77d785d9f17e129e8c3fd0d845b6d53a0f36ed1d00777f24f988be1d0
[CONFIG] 2022/07/21 12:22:55 Generated cookie seed:  d9e3f28e1ddfa7d4ed66d33d8425ba7232449aa9d4137567793fe557054cd42196721c94071ff3e7eec95cd77065c3cf3bb60847754a5acdc4d7961c15abe30c
[CONFIG] 2022/07/21 12:22:55 Generated csrf seed:    b68c6170214f744db5025283a1e92947ae27a8f90c07aae3e03864ee9bdb23df
[CONFIG] 2022/07/21 12:22:55 Writing <path>\authboss-worked\data\config\worked-config.yml.

````

#### Seed values

The seed values for `seed:session`, `seed:cookie` and `seed:csrf` _SHOULD NOT BE
CHANGED_ after the `worked_udata.sqlite3` database has been created. If you do
change them, existing sessions will become unusable.

  * The `genconfig.go` application generates the seed values so that they are
  not part of the code or visible in a source code repository such as Github or
  Gitlab.

  * If you do change the seed values. do one of the following:

    * Use `sqlite3` to delete all rows from the `sessions` table.

    * Delete the `worked_udata.sqlite3` database and re-create it. Deleting the
      `worked_udata.sqlite3` database will also remove any users you might have
      registered or added.

### Run the demo

The output should look similar to the log below. `authboss-worked` is
intentionally verbose. The `---` annotations indicate where the log was trimmed.

````
$ go run worked-main.go
[CONFIG] 2022/07/26 09:34:57 Reading <path>\authboss-worked\data\config\worked-config.yml.
[USERDB] 2022/07/26 09:34:57 userdb path <path>\authboss-worked\worked_udata.sqlite3
[USERDB] 2022/07/26 09:34:57 <gpkg>go/pkg/mod/github.com/glebarez/sqlite@v1.4.6/migrator.go:33
[1.053ms] [rows:-] SELECT count(*) FROM sqlite_master WHERE type='table' AND name="udata"
[USERDB] 2022/07/26 09:34:57 <gpkg>go/pkg/mod/github.com/glebarez/sqlite@v1.4.6/migrator.go:111
[0.527ms] [rows:4] SELECT sql FROM sqlite_master WHERE type IN ("table","index") AND tbl_name = "udata" AND sql IS NOT NULL order by type = "table" desc
--- ...
--- Lots of output from GORM database migration/creation
--- ...
[CONFIG] 2022/07/26 09:34:57 Loaded fragment template '_login_form' from content\fragments\_login_form.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded fragment template '_navbar' from content\fragments\_navbar.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'app_index' from content\app_index.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'confirm_html' from content\confirm_html.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'confirm_txt' from content\confirm_txt.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'index' from content\index.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'login' from content\login.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'recover_end' from content\recover_end.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'recover_html' from content\recover_html.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'recover_start' from content\recover_start.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'recover_txt' from content\recover_txt.gohtml
[CONFIG] 2022/07/26 09:34:57 Loaded HTML template 'register' from content\register.gohtml
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying login
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying confirm_html
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying confirm_txt
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying recover_start
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying recover_end
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying recover_html
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying recover_txt
[CONFIG] 2022/07/26 09:34:57 Templates.Load: Verifying register
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
- using env:   export GIN_MODE=release
- using code:  gin.SetMode(gin.ReleaseMode)

[GIN-debug] GET    /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] POST   /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] PUT    /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] PATCH  /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] HEAD   /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] OPTIONS /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] DELETE /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] CONNECT /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] TRACE  /auth/*wild               --> github.com/gin-gonic/gin.WrapH.func1 (8 handlers)
[GIN-debug] GET    /app/                     --> gitlab.com/scooter-phd/authboss-worked/abossworked.renderPageAsTemplate.func1 (11 handlers)
[GIN-debug] GET    /images/*filepath         --> github.com/gin-gonic/gin.(*RouterGroup).createStaticHandler.func1 (8 handlers)
[GIN-debug] HEAD   /images/*filepath         --> github.com/gin-gonic/gin.(*RouterGroup).createStaticHandler.func1 (8 handlers)
[GIN-debug] GET    /unauthorized             --> gitlab.com/scooter-phd/authboss-worked/abossworked.renderPageAsTemplate.func1 (8 handlers)
[GIN-debug] GET    /                         --> gitlab.com/scooter-phd/authboss-worked/abossworked.renderPageAsTemplate.func1 (8 handlers)
[GIN-debug] Listening and serving HTTP on localhost:3000

````

### Try the demo's functionality

Connect to the [demo's web server](http://localhost:3000/) -- the link will take
you to the default listen address, `localhost:3000`. If you changed this in the
YAML configuration, use your configured listen address.

#### Sign in as a nonexistent user

Try logging in as `foo@bar.com` and any random password. Since there are no
egistered users, you'll be taken to the ordinary sign-in page (the `/auth/login`
URL path).


#### Register and confirm a user

If the Authboss _confirm_ module is enabled, which is the default, check the
demo's log output for the confirmation URL that completes the new user's
registration. Copy and paste the confirmation URL in your browser.

Look for something similar to the output below in the demo log for the user
confirmation URL:

````
To: foo@bar.com
From:
Subject: Confirm New Account
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="===============284fad24nao8f4na284f2n4=="
Content-Transfer-Encoding: 7bit

--===============284fad24nao8f4na284f2n4==
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 7bit

Please copy and paste the following link into your browser to confirm your account\n\n<__COPY AND PASTE THIS CONFIRMATION URL__>

--===============284fad24nao8f4na284f2n4==
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: 7bit

<h1>
  Confirm your account
</h1>

<p>
  Please click <a href="<__COPY AND PASTE THIS CONFIRMATION URL__>">here</a> to confirm your account
</p>

--===============284fad24nao8f4na284f2n4==--
````

#### Sign in as a valid user.

Once you've confirmed the user successfully, you can now sign in as that user.
Notice that when you've signed in successfully, the "Authboss. Gin. Worked."
banner turns from red to green.

#### Lockout

Logout and return to the [top index page](http://localhost:3000/) with the login
form. Try incorrectly logging in three (3) times. After three incorrect logins,
the account should be locked. Wait for a little bit more than five (5) minutes
for the account to unlock and log in correctly.

#### Recover an account

If you're signed in, logout. Click on the _Recover!_ button, and enter the
user's e-mail address in the _Password/account recovery page_. Look for a
recovery URL in the log, copy and paste the recovery URL into your browser. If
you have successfully copied and pasted the recovery URL into your browser, the
recovery URL takes you to a form in which you can then update the account's
password.

The recovery URL "email" will show up in the log and looks similar to this:

````
To: foo@bar.com
From:
Subject: Password Reset
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="===============284fad24nao8f4na284f2n4=="
Content-Transfer-Encoding: 7bit

--===============284fad24nao8f4na284f2n4==
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 7bit

Please copy and paste the following link into your browser to recover your account\n\n<__COPY AND PASTE THIS RECOVERY URL__>

--===============284fad24nao8f4na284f2n4==
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: 7bit


<h1>
  Recover your account.
</h1>

<p>
  Please click <a href="<__COPY AND PASTE THIS RECOVERY URL__>">here</a> to recover your account
</p>

--===============284fad24nao8f4na284f2n4==--
````


## Where to go from here...

- Read the [code walkthrough](WALKTHROUGH.md)
- Read the code itself.


## What does "scooter me fecit" mean?

There was a time, not that long ago, when Classical Latin was regularly taught
in late elementary and early middle school.

"Me fecit" means "made me", pronounced "mee fake-it" in Classical Latin. It was
a common inscription on many items. (Eben: If you ever read this, yes, that was
a benefit of a southern Connecticut education.)


[gpl-licenses]: https://www.gnu.org/licenses/
[authboss]: https://github.com/volatiletech/authboss
[authboss_sample]: https://github.com/volatiletech/authboss-sample
[net-http]: https://pkg.go.dev/net/http
[go-chi]: https://go-chi.io/#/
[go-pkgs]: https://pkg.go.dev/
[html.template]: https://pkg.go.dev/html/template
[gin-gonic]: https://pkg.go.dev/github.com/gin-gonic/gin
[authboss-gin-sample]: https://github.com/jesusvazquez/authboss-gin-sample
[gorm.io]: https://gorm.io/
[go-yaml]: https://github.com/go-yaml/yaml
[gin-sessions]: https://github.com/gin-contrib/sessions
[gorm-sqlite]: https://pkg.go.dev/gorm.io/driver/sqlite
[gorilla-sessions]: https://pkg.go.dev/github.com/gorilla/sessions
