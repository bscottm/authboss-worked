CREATE TABLE IF NOT EXISTS udata
(
	-- User ID: This is a GUID string
	uid CHAR(64) NOT NULL PRIMARY KEY,
	-- User ID data (bcrypt-ed password)
	uid_data VARCHAR(64) NOT NULL,
	-- User e-mail address
	email VARCHAR NOT NULL ,
	-- Confirmed user flag
	confirmed INTEGER,
	-- Unsuccessful login attempts
	attempts INTEGER,
	-- Last attempted login time, ISO8601 string
	last_attempt_time TEXT,
	-- Time at which the account was locked, ISO8601 string
	locked_out_time TEXT,
	-- Recovery token's expiration time, ISO8601 string
	recovery_token_expire TEXT,

	-- OAUTH2:
	oauth2_uid VARCHAR,
	oauth2_provider VARCHAR,
	oauth2_access_token VARCHAR,
	oauth2_refresh_token VARCHAR,
	oauth2_expire TEXT,

	-- 2 Factor Authentication:
	totp_secret_key VARCHAR,
	sms_phone_number VARCHAR,
	sms_seed_phone_number VARCHAR,
	recovery_codes VARCHAR
);

CREATE UNIQUE INDEX IF NOT EXISTS udata_uid_data ON udata(uid_data);
CREATE UNIQUE INDEX IF NOT EXISTS udata_email ON udata(email);
CREATE UNIQUE INDEX IF NOT EXISTS udata_oauth2_uid ON udata(oauth2_uid);
