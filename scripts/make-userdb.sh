#!/bin/sh

sqlite3 explan_udata.sqlite3 < scripts/explan_udata.sql
sqlite3 explan_udata.sqlite3 "select sql from sqlite_schema;" | sha256sum
sqlite3 explan_udata.sqlite3 "select sql from sqlite_schema;" > yyy.sql