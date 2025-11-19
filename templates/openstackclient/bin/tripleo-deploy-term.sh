#!/usr/bin/env bash
umask 022

CONFIG_VERSION=${CONFIG_VERSION:?"Please set CONFIG_VERSION."}
RUNDIR="/var/run/tripleo-deploy/$CONFIG_VERSION"
PGIDFILE=$RUNDIR/pgid

if [[ ! -e $PGIDFILE ]]; then
    exit 0
fi

PGID=$(cat $PGIDFILE)

if kill -0 -$PGID; then
    echo "Terminating tripleo-deploy($PGID)"
    kill -- -$PGID
    i=0
    until [[ "$i" -gt 50 ]]; do
        kill -0 -$PGID && exit 0
        sleep 1
    done
    kill -9 -$PGID
fi
