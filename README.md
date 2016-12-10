## LazyFS (Lazy Filesystem)

LazyFS is a tool built on top of [CRIU](https://criu.org/Main_Page) to assist
process migration between hosts. LazyFS specifically addresses the issue of
file migration: currently you can either have both processes (pre-migration and
post migration) refer to the same NFS, or you can transfer all the necessary
files from one machine to another before restarting the migrated process. LazyFS
chooses to find the middleground and instead requests only the files it needs
from the original host at the time of a read or write access by the migrated
process.

LazyFS is implemented in GO as a [FUSE](https://en.wikipedia.org/wiki/Filesystem_in_Userspace) interface.
LazyFS relies on SCP currently to transfer images from the original host to the
new (post-migration) host. For migration to run smoothly, it is recommended that
you setup SSH keys between the hosts before migration.

### A video demo of LazyFS using read

[![Read Demo](https://img.youtube.com/vi/5fqaI-HCDDI/0.jpg)](https://youtu.be/5fqaI-HCDDI)

### A video demo of LazyFS using write

[![Write Demo](https://img.youtube.com/vi/kQdNOy8ENX8/0.jpg)](https://youtu.be/kQdNOy8ENX8)
