# vfs - fork of pebble/vfs

This is a fork and extension of the vfs mock file system
test affordance from Pebble by Cockroach Labs.

https://github.com/cockroachdb/pebble/

The original lacked the Truncate method necessary to 
test write-ahead-logs that are truncated to a specific
size on recovery.

LICENSE: see the LICENSE files herein. 

(parts are MIT licensed, parts are Apache 2 licensed, and the
top level is BSD 3-clause licensed)
