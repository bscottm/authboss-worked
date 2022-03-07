#!/bin/sh

if ! docker volume inspect explan_data 2>&1 > /dev/null; then
  echo "-- Creating explan_data over NFS on arm-twist"
  docker volume create --driver local \
    --opt type=nfs \
    --opt o=addr=bigyard,rw \
    --opt device=:arm-twist/explan_data \
    explan_data
else
  echo "-- explan_data exists."
fi


