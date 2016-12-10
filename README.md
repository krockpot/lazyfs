# LazyFS (Lazy Filesystem)

LazyFS is a tool built on top of [CRIU](https://criu.org/Main_Page) to assist
process migration between hosts. LazyFS specifically addresses the issue of
file migration: currently you can either have both processes (pre-migration and
post migration) refer to the same NFS, or you can transfer all the necessary
files from one machine to another before restarting the migrated process. LazyFS
chooses to find the middleground and instead requests only the files it needs
from the original host at the time of a read or write access by the migrated
process.

LazyFS is implemented in Go as a [FUSE](https://en.wikipedia.org/wiki/Filesystem_in_Userspace) interface.
LazyFS relies on SCP currently to transfer images from the original host to the
new (post-migration) host. For migration to run smoothly, it is recommended that
you setup SSH keys between the hosts before migration.

## Installation

Install Go and all [dependencies for CRIU](https://criu.org/Installation).

Clone my [custom version of CRIU](https://github.com/jakrach/criu) and continue
with installation steps from link above. This version simply has a hook on
restoration that checks if a file already exists locally: if it does not, it
instead opens /lazyfs/(original filename with '.' replacing '/').

Clone LazyFS and compile the CRIU protobuf scheme into Go. In the root
for this project run `protoc --go_out=. protobuf/regfile.proto`.

Finally, from the root of this project, run `go install`.

## Usage

## Demos

### A video demo of LazyFS using read

[![Read Demo](https://img.youtube.com/vi/5fqaI-HCDDI/0.jpg)](https://youtu.be/5fqaI-HCDDI)

### A video demo of LazyFS using write

[![Write Demo](https://img.youtube.com/vi/kQdNOy8ENX8/0.jpg)](https://youtu.be/kQdNOy8ENX8)
